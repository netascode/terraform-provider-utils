// Copyright © 2022 Cisco Systems, Inc. and its affiliates.
// All rights reserved.
//
// Licensed under the Mozilla Public License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://mozilla.org/MPL/2.0/
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"math/big"
	"net"
	"strings"

	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/gocty"
)

// IP network template functions with the behavior of the Terraform functions of
// the same name (cidrhost, cidrnetmask, cidrsubnet, cidrsubnets) and of the
// OpenTofu function cidrcontains, which Terraform does not have.
//
// Like Terraform, addresses are parsed with the rules of Go 1.16 (decimal IPv4
// octets with leading zeros are accepted, e.g. "010.001.0.0/24" is 10.1.0.0/24)
// and results are formatted like Go's net.IP and net.IPNet (IPv4-mapped IPv6
// addresses as dotted IPv4).
//
// Intentional differences to Terraform, where Terraform returns wrong results:
// host and subnet numbers that do not fit the prefix (including negative
// newbits and netnum, and numbers beyond 64 bits) are errors instead of
// addresses outside the prefix or panics, cidrhost handles IPv4-mapped IPv6
// prefixes, cidrsubnets can allocate subnets at the start of the address space
// (0.0.0.0, ::), and the error of a negative host number names that number. A
// negative newbits of cidrsubnet has its own error.

// cidrParseError is the error of an invalid prefix, worded like the parse error
// Terraform reports ("invalid CIDR address: <prefix>").
func cidrParseError(s string) error {
	return &net.ParseError{Type: "CIDR address", Text: s}
}

// cidrDecimal parses the leading decimal digits of s, returning the number, the
// number of digits and whether there was at least one digit and no overflow.
func cidrDecimal(s string) (int, int, bool) {
	const limit = 0xFFFFFF
	n, i := 0, 0
	for ; i < len(s) && s[i] >= '0' && s[i] <= '9'; i++ {
		n = n*10 + int(s[i]-'0')
		if n >= limit {
			return limit, i, false
		}
	}
	return n, i, i > 0
}

// parseCIDRIPv4 parses a dotted IPv4 address with exactly four decimal octets
// (leading zeros allowed) into a 4-byte IP, or returns nil.
func parseCIDRIPv4(s string) net.IP {
	ip := make(net.IP, net.IPv4len)
	for i := 0; i < net.IPv4len; i++ {
		if i > 0 {
			if len(s) == 0 || s[0] != '.' {
				return nil
			}
			s = s[1:]
		}
		n, c, ok := cidrDecimal(s)
		if !ok || n > 0xFF {
			return nil
		}
		ip[i] = byte(n)
		s = s[c:]
	}
	if len(s) != 0 {
		return nil
	}
	return ip
}

// parseCIDRIPv6 parses an IPv6 address (without zone) into a 16-byte IP, or
// returns nil. An embedded IPv4 address may have octets with leading zeros.
func parseCIDRIPv6(s string) net.IP {
	if !strings.Contains(s, ":") || strings.Contains(s, "%") {
		return nil
	}
	if i := strings.LastIndexByte(s, ':'); strings.Contains(s[i+1:], ".") {
		v4 := parseCIDRIPv4(s[i+1:])
		if v4 == nil {
			return nil
		}
		s = s[:i+1] + v4.String()
	}
	ip := net.ParseIP(s)
	if ip == nil {
		return nil
	}
	return ip.To16()
}

// parseCIDRIP parses an IPv4 or IPv6 address, or returns nil. IPv4 addresses
// are returned as 16-byte IPs, like net.ParseIP.
func parseCIDRIP(s string) net.IP {
	if ip := parseCIDRIPv4(s); ip != nil {
		return ip.To16()
	}
	return parseCIDRIPv6(s)
}

// parseCIDR parses a prefix in CIDR notation and returns the network (the
// address with the host bits cleared, 4 bytes for IPv4 and 16 bytes for IPv6).
func parseCIDR(s string) (*net.IPNet, error) {
	i := strings.IndexByte(s, '/')
	if i < 0 {
		return nil, cidrParseError(s)
	}
	addr, mask := s[:i], s[i+1:]
	bits := 8 * net.IPv4len
	ip := parseCIDRIPv4(addr)
	if ip == nil {
		bits = 8 * net.IPv6len
		ip = parseCIDRIPv6(addr)
	}
	ones, c, ok := cidrDecimal(mask)
	if ip == nil || !ok || c != len(mask) || ones > bits {
		return nil, cidrParseError(s)
	}
	m := net.CIDRMask(ones, bits)
	return &net.IPNet{IP: ip.Mask(m), Mask: m}, nil
}

// cidrAddress returns the address of network plus offset, keeping the address
// length of the network.
func cidrAddress(network *net.IPNet, offset *big.Int) net.IP {
	v := new(big.Int).SetBytes(network.IP)
	v.Add(v, offset)
	return net.IP(v.FillBytes(make([]byte, len(network.IP))))
}

// cidrBits returns the prefix length and address length of a network.
func cidrBits(network *net.IPNet) (int, int) {
	return network.Mask.Size()
}

// cidrSubnet returns the subnet numbered netnum of network extended by newbits.
func cidrSubnet(network *net.IPNet, newbits int, netnum *big.Int) (*net.IPNet, error) {
	ones, bits := cidrBits(network)
	if newbits < 0 {
		return nil, fmt.Errorf("must extend prefix by at least zero bits, not %d", newbits)
	}
	// Compared without adding to ones, which would overflow for large newbits.
	if newbits > bits-ones {
		return nil, fmt.Errorf("insufficient address space to extend prefix of %d by %d", ones, newbits)
	}
	count := new(big.Int).Lsh(big.NewInt(1), uint(newbits))
	if netnum.Sign() < 0 || netnum.Cmp(count) >= 0 {
		return nil, fmt.Errorf("prefix extension of %d does not accommodate a subnet numbered %s", newbits, netnum)
	}
	offset := new(big.Int).Lsh(netnum, uint(bits-ones-newbits))
	return &net.IPNet{IP: cidrAddress(network, offset), Mask: net.CIDRMask(ones+newbits, bits)}, nil
}

// cidrHostFunc calculates a host address within a prefix: hostnum 0 is the
// network address, negative numbers count back from the last address of the
// prefix (-1 is the last address).
var cidrHostFunc = function.New(&function.Spec{
	Description: "Calculates a full host IP address for a given host number within a given IP network address prefix.",
	Params: []function.Parameter{
		{Name: "prefix", Type: cty.String},
		{Name: "hostnum", Type: cty.Number},
	},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
		var hostnum *big.Int
		if err := gocty.FromCtyValue(args[1], &hostnum); err != nil {
			return cty.UnknownVal(cty.String), err
		}
		network, err := parseCIDR(args[0].AsString())
		if err != nil {
			return cty.UnknownVal(cty.String), fmt.Errorf("invalid CIDR expression: %s", err)
		}
		ones, bits := cidrBits(network)
		size := new(big.Int).Lsh(big.NewInt(1), uint(bits-ones))
		offset := new(big.Int).Set(hostnum)
		if offset.Sign() < 0 {
			offset.Add(offset, size)
		}
		if offset.Sign() < 0 || offset.Cmp(size) >= 0 {
			return cty.UnknownVal(cty.String), fmt.Errorf("prefix of %d does not accommodate a host numbered %s", ones, hostnum)
		}
		return cty.StringVal(cidrAddress(network, offset).String()), nil
	},
})

// cidrNetmaskFunc converts an IPv4 prefix into a subnet mask address.
var cidrNetmaskFunc = function.New(&function.Spec{
	Description: "Converts an IPv4 address prefix given in CIDR notation into a subnet mask address.",
	Params: []function.Parameter{
		{Name: "prefix", Type: cty.String},
	},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
		network, err := parseCIDR(args[0].AsString())
		if err != nil {
			return cty.UnknownVal(cty.String), fmt.Errorf("invalid CIDR expression: %s", err)
		}
		if network.IP.To4() == nil {
			return cty.UnknownVal(cty.String), fmt.Errorf("IPv6 addresses cannot have a netmask: %s", args[0].AsString())
		}
		return cty.StringVal(net.IP(network.Mask).String()), nil
	},
})

// cidrSubnetFunc calculates the subnet numbered netnum of a prefix extended by
// newbits bits.
var cidrSubnetFunc = function.New(&function.Spec{
	Description: "Calculates a subnet address within a given IP network address prefix.",
	Params: []function.Parameter{
		{Name: "prefix", Type: cty.String},
		{Name: "newbits", Type: cty.Number},
		{Name: "netnum", Type: cty.Number},
	},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
		var newbits int
		if err := gocty.FromCtyValue(args[1], &newbits); err != nil {
			return cty.UnknownVal(cty.String), err
		}
		var netnum *big.Int
		if err := gocty.FromCtyValue(args[2], &netnum); err != nil {
			return cty.UnknownVal(cty.String), err
		}
		network, err := parseCIDR(args[0].AsString())
		if err != nil {
			return cty.UnknownVal(cty.String), fmt.Errorf("invalid CIDR expression: %s", err)
		}
		subnet, err := cidrSubnet(network, newbits, netnum)
		if err != nil {
			return cty.UnknownVal(cty.String), err
		}
		return cty.StringVal(subnet.String()), nil
	},
})

// cidrSubnetsFunc allocates consecutive subnets of a prefix, one per newbits
// argument, each aligned to its own size (so a larger subnet after a smaller
// one skips the addresses up to its boundary).
var cidrSubnetsFunc = function.New(&function.Spec{
	Description: "Calculates a sequence of consecutive IP address ranges within a particular CIDR prefix.",
	Params: []function.Parameter{
		{Name: "prefix", Type: cty.String},
	},
	VarParam: &function.Parameter{Name: "newbits", Type: cty.Number},
	Type:     function.StaticReturnType(cty.List(cty.String)),
	Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
		network, err := parseCIDR(args[0].AsString())
		if err != nil {
			return cty.UnknownVal(cty.List(cty.String)), function.NewArgErrorf(0, "invalid CIDR expression: %s", err)
		}
		if len(args) == 1 {
			return cty.ListValEmpty(cty.String), nil
		}
		ones, bits := cidrBits(network)
		protocol := "IPv4"
		if bits == 8*net.IPv6len {
			protocol = "IPv6"
		}
		base := new(big.Int).SetBytes(network.IP)
		end := new(big.Int).Add(base, new(big.Int).Lsh(big.NewInt(1), uint(bits-ones)))
		next := new(big.Int).Set(base) // first address not yet allocated
		previous := ""                 // previous subnet, for error messages
		result := make([]cty.Value, 0, len(args)-1)
		for i, arg := range args[1:] {
			var newbits int
			if err := gocty.FromCtyValue(arg, &newbits); err != nil {
				return cty.UnknownVal(cty.List(cty.String)), function.NewArgError(i+1, err)
			}
			if newbits < 1 {
				return cty.UnknownVal(cty.List(cty.String)), function.NewArgErrorf(i+1, "must extend prefix by at least one bit")
			}
			if newbits > 32 {
				return cty.UnknownVal(cty.List(cty.String)), function.NewArgErrorf(i+1, "may not extend prefix by more than 32 bits")
			}
			length := ones + newbits
			if length > bits {
				return cty.UnknownVal(cty.List(cty.String)), function.NewArgErrorf(i+1, "would extend prefix to %d bits, which is too long for an %s address", length, protocol)
			}
			// Align the next free address up to the size of this subnet.
			size := new(big.Int).Lsh(big.NewInt(1), uint(bits-length))
			start := new(big.Int).Add(next, new(big.Int).Sub(size, big.NewInt(1)))
			start.Div(start, size).Mul(start, size)
			// The first subnet always fits, as it starts at the base of the
			// prefix, so previous is set when this fails.
			if new(big.Int).Add(start, size).Cmp(end) > 0 {
				return cty.UnknownVal(cty.List(cty.String)), function.NewArgErrorf(i+1, "not enough remaining address space for a subnet with a prefix of %d bits after %s", length, previous)
			}
			subnet := &net.IPNet{
				IP:   cidrAddress(network, new(big.Int).Sub(start, base)),
				Mask: net.CIDRMask(length, bits),
			}
			previous = subnet.String()
			result = append(result, cty.StringVal(previous))
			next.Add(start, size)
		}
		return cty.ListVal(result), nil
	},
})

// cidrContainsFunc returns whether an IP address or prefix lies within a prefix
// (the OpenTofu function cidrcontains). Address families must match.
var cidrContainsFunc = function.New(&function.Spec{
	Description: "Determines whether a given IP address or an address prefix given in CIDR notation is within a given IP network address prefix.",
	Params: []function.Parameter{
		{Name: "containing_prefix", Type: cty.String},
		{Name: "contained_ip_or_prefix", Type: cty.String},
	},
	Type: function.StaticReturnType(cty.Bool),
	Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
		prefix, addr := args[0].AsString(), args[1].AsString()
		containing, err := parseCIDR(prefix)
		if err != nil {
			return cty.UnknownVal(cty.Bool), err
		}
		first := parseCIDRIP(addr)
		var last net.IP
		if first == nil {
			contained, err := parseCIDR(addr)
			if err != nil {
				return cty.UnknownVal(cty.Bool), fmt.Errorf("invalid IP address or prefix: %s", addr)
			}
			ones, bits := cidrBits(contained)
			first = contained.IP
			last = cidrAddress(contained, new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(bits-ones)), big.NewInt(1)))
		}
		if (first.To4() == nil) != (containing.IP.To4() == nil) {
			return cty.UnknownVal(cty.Bool), fmt.Errorf("address family mismatch: %s vs. %s", prefix, addr)
		}
		result := containing.Contains(first)
		if last != nil {
			result = result && containing.Contains(last)
		}
		return cty.BoolVal(result), nil
	},
})
