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
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Format constants for BGP RD type detection (RFC 4364)
const (
	FormatBgpRdAuto        = "auto"         // BGP RD auto-assignment
	FormatBgpRdTwoByteAS   = "two_byte_as"  // Type 0: 2-byte AS number (1-65535)
	FormatBgpRdFourByteAS  = "four_byte_as" // Type 2: 4-byte AS number (65536-4294967295)
	FormatBgpRdIPv4Address = "ipv4_address" // Type 1: IPv4 address
)

// BGP RD range constants per RFC 4364
const (
	MinRdTwoByteAS              uint64 = 1
	MaxRdTwoByteAS              uint64 = 65535
	MinRdFourByteAS             uint64 = 65536
	MaxRdFourByteAS             uint64 = 4294967295
	MaxRdTwoByteAssignedNumber  uint64 = 4294967295
	MaxRdFourByteAssignedNumber uint64 = 65535
	MaxRdIPv4AssignedNumber     uint64 = 65535
)

var _ function.Function = NormalizeBgpRdFunction{}

// normalizeBgpRd parses a BGP RD colon notation string and detects the format type.
// Enforces RFC range constraints: two-byte AS (1-65535, assigned 0-4294967295),
// four-byte AS (65536-4294967295, assigned 0-65535), IPv4 (assigned 0-65535).
func normalizeBgpRd(value string) (format string, asNumber int64, assignedNumber int64, ipv4Address string, err error) {
	if strings.EqualFold(strings.TrimSpace(value), "auto") {
		return FormatBgpRdAuto, 0, 0, "", nil
	}

	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 {
		return "", 0, 0, "", fmt.Errorf("invalid BGP RD format '%s': expected 'X:Y' colon notation", value)
	}

	left, right := parts[0], parts[1]

	rightNum, err := strconv.ParseUint(right, 10, 64)
	if err != nil {
		return "", 0, 0, "", fmt.Errorf("invalid assigned number '%s' in '%s': must be a non-negative integer", right, value)
	}

	if strings.Contains(left, ".") {
		ip := net.ParseIP(left)
		if ip == nil || ip.To4() == nil {
			return "", 0, 0, "", fmt.Errorf("invalid IPv4 address '%s' in '%s': must be a valid IPv4 address", left, value)
		}
		if rightNum > MaxRdIPv4AssignedNumber {
			return "", 0, 0, "", fmt.Errorf("invalid assigned number %d in '%s': IPv4 address format assigned number must be 0-%d", rightNum, value, MaxRdIPv4AssignedNumber)
		}
		return FormatBgpRdIPv4Address, 0, int64(rightNum), left, nil
	}

	leftNum, err := strconv.ParseUint(left, 10, 64)
	if err != nil {
		return "", 0, 0, "", fmt.Errorf("invalid AS number '%s' in '%s': must be a non-negative integer or IPv4 address", left, value)
	}

	if leftNum > MaxRdTwoByteAS {
		if leftNum > MaxRdFourByteAS {
			return "", 0, 0, "", fmt.Errorf("invalid AS number %d in '%s': four-byte AS number must be %d-%d", leftNum, value, MinRdFourByteAS, MaxRdFourByteAS)
		}
		if rightNum > MaxRdFourByteAssignedNumber {
			return "", 0, 0, "", fmt.Errorf("invalid assigned number %d in '%s': four-byte AS format assigned number must be 0-%d", rightNum, value, MaxRdFourByteAssignedNumber)
		}
		return FormatBgpRdFourByteAS, int64(leftNum), int64(rightNum), "", nil
	}

	if leftNum < MinRdTwoByteAS {
		return "", 0, 0, "", fmt.Errorf("invalid AS number %d in '%s': two-byte AS number must be %d-%d", leftNum, value, MinRdTwoByteAS, MaxRdTwoByteAS)
	}
	if rightNum > MaxRdTwoByteAssignedNumber {
		return "", 0, 0, "", fmt.Errorf("invalid assigned number %d in '%s': two-byte AS format assigned number must be 0-%d", rightNum, value, MaxRdTwoByteAssignedNumber)
	}

	return FormatBgpRdTwoByteAS, int64(leftNum), int64(rightNum), "", nil
}

func NewNormalizeBgpRdFunction() function.Function {
	return &NormalizeBgpRdFunction{}
}

type NormalizeBgpRdFunction struct{}

func (r NormalizeBgpRdFunction) Metadata(_ context.Context, req function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "normalize_bgp_rd"
}

func (r NormalizeBgpRdFunction) Definition(_ context.Context, _ function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary:             "Normalize a BGP Route Distinguisher from colon notation",
		MarkdownDescription: "Takes a BGP RD in standard colon notation (e.g., '65000:1001', '192.168.100.1:1002', '4200000001:1003') or the keyword 'auto' and returns an object with the detected format type and parsed components. Supports four format types: 'auto' (BGP RD auto-assignment), 'two_byte_as' (AS <= 65535), 'four_byte_as' (AS > 65535), and 'ipv4_address' (IPv4:value).\n\n" +
			"## Return Object\n\n" +
			"The function returns an object with the following attributes:\n\n" +
			"| Attribute | Type | Description |\n" +
			"|-----------|------|-------------|\n" +
			"| `format` | String | Detected format: `\"auto\"`, `\"two_byte_as\"`, `\"four_byte_as\"`, or `\"ipv4_address\"` |\n" +
			"| `as_number` | Number | Administrator subfield as AS number (populated for `two_byte_as` and `four_byte_as`; `0` for `ipv4_address` and `auto`) |\n" +
			"| `assigned_number` | Number | Assigned Number subfield (always populated; `0` for `auto`) |\n" +
			"| `ipv4_address` | String | Administrator subfield as IPv4 address (populated for `ipv4_address`; `\"\"` for AS formats and `auto`) |",
		Parameters: []function.Parameter{
			function.StringParameter{
				Name:                "value",
				MarkdownDescription: "BGP RD in colon notation (e.g., '65000:1001', '192.168.100.1:1002', '4200000001:1003') or the keyword 'auto'.",
			},
		},
		Return: function.ObjectReturn{
			AttributeTypes: map[string]attr.Type{
				"format":          types.StringType,
				"as_number":       types.Int64Type,
				"assigned_number": types.Int64Type,
				"ipv4_address":    types.StringType,
			},
		},
	}
}

func (r NormalizeBgpRdFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var inputValue string

	resp.Error = function.ConcatFuncErrors(req.Arguments.Get(ctx, &inputValue))
	if resp.Error != nil {
		return
	}

	format, asNumber, assignedNumber, ipv4Address, err := normalizeBgpRd(inputValue)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError(err.Error()))
		return
	}

	result, diags := types.ObjectValue(
		map[string]attr.Type{
			"format":          types.StringType,
			"as_number":       types.Int64Type,
			"assigned_number": types.Int64Type,
			"ipv4_address":    types.StringType,
		},
		map[string]attr.Value{
			"format":          types.StringValue(format),
			"as_number":       types.Int64Value(asNumber),
			"assigned_number": types.Int64Value(assignedNumber),
			"ipv4_address":    types.StringValue(ipv4Address),
		},
	)
	if diags.HasError() {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError(fmt.Sprintf("failed to construct result object: %s", diags)))
		return
	}

	resp.Error = function.ConcatFuncErrors(resp.Result.Set(ctx, result))
}
