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

// Format constants for BGP RD/RT type detection (RFC 4364 / RFC 4360)
const (
	FormatBgpTwoByteAS  = "two_byte_as"  // Type 0: 2-byte AS number (1-65535)
	FormatBgpFourByteAS = "four_byte_as" // Type 2: 4-byte AS number (65536-4294967295)
	FormatBgpIPv4Address = "ipv4_address" // Type 1: IPv4 address
)

// BGP RD/RT range constants per RFC 4364 / RFC 4360
const (
	MinTwoByteAS             = 1
	MaxTwoByteAS             = 65535
	MinFourByteAS            = 65536
	MaxFourByteAS            = 4294967295
	MaxTwoByteAssignedNumber = 4294967295
	MaxFourByteAssignedNumber = 65535
	MaxIPv4AssignedNumber    = 65535
)

var _ function.Function = ParseBgpRdRtFunction{}

// parseBgpRdRt parses a BGP RD/RT colon notation string and detects the format type.
// Enforces RFC range constraints: two-byte AS (1-65535, assigned 0-4294967295),
// four-byte AS (65536-4294967295, assigned 0-65535), IPv4 (assigned 0-65535).
func parseBgpRdRt(value string) (format string, asNumber int64, assignedNumber int64, ipv4Address string, err error) {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 {
		return "", 0, 0, "", fmt.Errorf("invalid BGP RD/RT format '%s': expected 'X:Y' colon notation", value)
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
		if rightNum > MaxIPv4AssignedNumber {
			return "", 0, 0, "", fmt.Errorf("invalid assigned number %d in '%s': IPv4 address format assigned number must be 0-%d", rightNum, value, MaxIPv4AssignedNumber)
		}
		return FormatBgpIPv4Address, 0, int64(rightNum), left, nil
	}

	leftNum, err := strconv.ParseUint(left, 10, 64)
	if err != nil {
		return "", 0, 0, "", fmt.Errorf("invalid AS number '%s' in '%s': must be a non-negative integer or IPv4 address", left, value)
	}

	if leftNum > MaxTwoByteAS {
		if leftNum > MaxFourByteAS {
			return "", 0, 0, "", fmt.Errorf("invalid AS number %d in '%s': four-byte AS number must be %d-%d", leftNum, value, MinFourByteAS, MaxFourByteAS)
		}
		if rightNum > MaxFourByteAssignedNumber {
			return "", 0, 0, "", fmt.Errorf("invalid assigned number %d in '%s': four-byte AS format assigned number must be 0-%d", rightNum, value, MaxFourByteAssignedNumber)
		}
		return FormatBgpFourByteAS, int64(leftNum), int64(rightNum), "", nil
	}

	if leftNum < MinTwoByteAS {
		return "", 0, 0, "", fmt.Errorf("invalid AS number %d in '%s': two-byte AS number must be %d-%d", leftNum, value, MinTwoByteAS, MaxTwoByteAS)
	}
	if rightNum > MaxTwoByteAssignedNumber {
		return "", 0, 0, "", fmt.Errorf("invalid assigned number %d in '%s': two-byte AS format assigned number must be 0-%d", rightNum, value, MaxTwoByteAssignedNumber)
	}

	return FormatBgpTwoByteAS, int64(leftNum), int64(rightNum), "", nil
}

func NewParseBgpRdRtFunction() function.Function {
	return &ParseBgpRdRtFunction{}
}

type ParseBgpRdRtFunction struct{}

func (r ParseBgpRdRtFunction) Metadata(_ context.Context, req function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "parse_bgp_rd_rt"
}

func (r ParseBgpRdRtFunction) Definition(_ context.Context, _ function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary:             "Parse a BGP Route Distinguisher or Route Target from colon notation",
		MarkdownDescription: "Takes a BGP RD/RT in standard colon notation (e.g., '65000:1001', '192.168.100.1:1002', '4200000001:1003') and returns an object with the detected format type and parsed components. Supports three format types per RFC 4364/4360: 'two_byte_as' (AS <= 65535), 'four_byte_as' (AS > 65535), and 'ipv4_address' (IPv4:value).",
		Parameters: []function.Parameter{
			function.StringParameter{
				Name:                "value",
				MarkdownDescription: "BGP RD/RT in colon notation (e.g., '65000:1001', '192.168.100.1:1002', '4200000001:1003').",
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

func (r ParseBgpRdRtFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var inputValue string

	resp.Error = function.ConcatFuncErrors(req.Arguments.Get(ctx, &inputValue))
	if resp.Error != nil {
		return
	}

	format, asNumber, assignedNumber, ipv4Address, err := parseBgpRdRt(inputValue)
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
