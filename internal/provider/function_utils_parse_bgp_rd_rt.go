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

// BGP RD/RT range constants
const (
	MaxTwoByteAS = 65535
)

var _ function.Function = ParseBgpRdRtFunction{}

// parseBgpRdRt parses a BGP RD/RT colon notation string and detects the format type
func parseBgpRdRt(value string) (format string, asNumber int64, assignedNumber int64, ipv4Address string, err error) {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 {
		return "", 0, 0, "", fmt.Errorf("invalid BGP RD/RT format '%s': expected 'X:Y' colon notation", value)
	}

	left, right := parts[0], parts[1]

	rightNum, err := strconv.ParseInt(right, 10, 64)
	if err != nil {
		return "", 0, 0, "", fmt.Errorf("invalid assigned number '%s' in '%s': must be a valid integer", right, value)
	}

	if strings.Contains(left, ".") {
		ip := net.ParseIP(left)
		if ip == nil || ip.To4() == nil {
			return "", 0, 0, "", fmt.Errorf("invalid IPv4 address '%s' in '%s': must be a valid IPv4 address", left, value)
		}
		return FormatBgpIPv4Address, 0, rightNum, left, nil
	}

	leftNum, err := strconv.ParseInt(left, 10, 64)
	if err != nil {
		return "", 0, 0, "", fmt.Errorf("invalid AS number '%s' in '%s': must be a valid integer or IPv4 address", left, value)
	}

	if leftNum > MaxTwoByteAS {
		return FormatBgpFourByteAS, leftNum, rightNum, "", nil
	}

	return FormatBgpTwoByteAS, leftNum, rightNum, "", nil
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
