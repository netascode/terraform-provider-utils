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

// Format constants for BGP RT type detection (RFC 4360)
const (
	FormatBgpRtAuto        = "auto"         // BGP RT auto-assignment
	FormatBgpRtTwoByteAS   = "two_byte_as"  // Type 0: 2-byte AS number (1-65535)
	FormatBgpRtFourByteAS  = "four_byte_as" // Type 2: 4-byte AS number (65536-4294967295)
	FormatBgpRtIPv4Address = "ipv4_address" // Type 1: IPv4 address
)

// BGP RT range constants per RFC 4360
const (
	MinRtTwoByteAS              = 1
	MaxRtTwoByteAS              = 65535
	MinRtFourByteAS             = 65536
	MaxRtFourByteAS             = 4294967295
	MaxRtTwoByteAssignedNumber  = 4294967295
	MaxRtFourByteAssignedNumber = 65535
	MaxRtIPv4AssignedNumber     = 65535
)

var _ function.Function = NormalizeBgpRtFunction{}

// normalizeBgpRt parses a BGP RT colon notation string and detects the format type.
// Enforces RFC range constraints: two-byte AS (1-65535, assigned 0-4294967295),
// four-byte AS (65536-4294967295, assigned 0-65535), IPv4 (assigned 0-65535).
func normalizeBgpRt(value string) (format string, asNumber int64, assignedNumber int64, ipv4Address string, err error) {
	if strings.EqualFold(strings.TrimSpace(value), "auto") {
		return FormatBgpRtAuto, 0, 0, "", nil
	}

	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 {
		return "", 0, 0, "", fmt.Errorf("invalid BGP RT format '%s': expected 'X:Y' colon notation", value)
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
		if rightNum > MaxRtIPv4AssignedNumber {
			return "", 0, 0, "", fmt.Errorf("invalid assigned number %d in '%s': IPv4 address format assigned number must be 0-%d", rightNum, value, MaxRtIPv4AssignedNumber)
		}
		return FormatBgpRtIPv4Address, 0, int64(rightNum), left, nil
	}

	leftNum, err := strconv.ParseUint(left, 10, 64)
	if err != nil {
		return "", 0, 0, "", fmt.Errorf("invalid AS number '%s' in '%s': must be a non-negative integer or IPv4 address", left, value)
	}

	if leftNum > MaxRtTwoByteAS {
		if leftNum > MaxRtFourByteAS {
			return "", 0, 0, "", fmt.Errorf("invalid AS number %d in '%s': four-byte AS number must be %d-%d", leftNum, value, MinRtFourByteAS, MaxRtFourByteAS)
		}
		if rightNum > MaxRtFourByteAssignedNumber {
			return "", 0, 0, "", fmt.Errorf("invalid assigned number %d in '%s': four-byte AS format assigned number must be 0-%d", rightNum, value, MaxRtFourByteAssignedNumber)
		}
		return FormatBgpRtFourByteAS, int64(leftNum), int64(rightNum), "", nil
	}

	if leftNum < MinRtTwoByteAS {
		return "", 0, 0, "", fmt.Errorf("invalid AS number %d in '%s': two-byte AS number must be %d-%d", leftNum, value, MinRtTwoByteAS, MaxRtTwoByteAS)
	}
	if rightNum > MaxRtTwoByteAssignedNumber {
		return "", 0, 0, "", fmt.Errorf("invalid assigned number %d in '%s': two-byte AS format assigned number must be 0-%d", rightNum, value, MaxRtTwoByteAssignedNumber)
	}

	return FormatBgpRtTwoByteAS, int64(leftNum), int64(rightNum), "", nil
}

func NewNormalizeBgpRtFunction() function.Function {
	return &NormalizeBgpRtFunction{}
}

type NormalizeBgpRtFunction struct{}

func (r NormalizeBgpRtFunction) Metadata(_ context.Context, req function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "normalize_bgp_rt"
}

func (r NormalizeBgpRtFunction) Definition(_ context.Context, _ function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary:             "Normalize a BGP Route Target from colon notation",
		MarkdownDescription: "Takes a BGP RT in standard colon notation (e.g., '65000:1001', '192.168.100.1:1002', '4200000001:1003') or the keyword 'auto' and returns an object with the detected format type and parsed components. Supports four format types: 'auto' (BGP RT auto-assignment), 'two_byte_as' (AS <= 65535), 'four_byte_as' (AS > 65535), and 'ipv4_address' (IPv4:value).\n\n" +
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
				MarkdownDescription: "BGP RT in colon notation (e.g., '65000:1001', '192.168.100.1:1002', '4200000001:1003') or the keyword 'auto'.",
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

func (r NormalizeBgpRtFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var inputValue string

	resp.Error = function.ConcatFuncErrors(req.Arguments.Get(ctx, &inputValue))
	if resp.Error != nil {
		return
	}

	format, asNumber, assignedNumber, ipv4Address, err := normalizeBgpRt(inputValue)
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
