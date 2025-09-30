package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Format constants to replace magic strings
const (
	FormatDottedDecimal = "dotted-decimal"
)

// ValidMaskFormats defines valid format values
var ValidMaskFormats = map[string]bool{
	FormatDottedDecimal: true,
}

// Mask range constants
const (
	MinMaskPrefixLength = 0
	MaxMaskPrefixLength = 32
)

var _ function.Function = NormalizeMaskFunction{}

// validateMaskPrefix validates a mask prefix length is within valid range (0-32)
func validateMaskPrefix(prefix int64) error {
	if prefix < MinMaskPrefixLength || prefix > MaxMaskPrefixLength {
		return fmt.Errorf("mask prefix length %d is out of valid range (%d-%d)", prefix, MinMaskPrefixLength, MaxMaskPrefixLength)
	}
	return nil
}

// convertPrefixToDottedDecimal converts a prefix length (0-32) to dotted-decimal notation
func convertPrefixToDottedDecimal(prefix int64) string {
	// Create a 32-bit mask with the first 'prefix' bits set to 1
	var mask uint32
	if prefix > 0 {
		mask = ^uint32(0) << (32 - prefix)
	}

	// Extract the four octets
	octet1 := (mask >> 24) & 0xFF
	octet2 := (mask >> 16) & 0xFF
	octet3 := (mask >> 8) & 0xFF
	octet4 := mask & 0xFF

	return fmt.Sprintf("%d.%d.%d.%d", octet1, octet2, octet3, octet4)
}

func NewNormalizeMaskFunction() function.Function {
	return &NormalizeMaskFunction{}
}

type NormalizeMaskFunction struct{}

func (r NormalizeMaskFunction) Metadata(_ context.Context, req function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "normalize_mask"
}

func (r NormalizeMaskFunction) Definition(_ context.Context, _ function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary:             "Normalize subnet mask from prefix length to specified format",
		MarkdownDescription: "Takes a subnet mask in prefix length format (0-32) and a format parameter. Returns the mask in the specified format. Currently supports 'dotted-decimal' format (e.g., 24 → '255.255.255.0').",
		Parameters: []function.Parameter{
			function.NumberParameter{
				Name:                "mask",
				MarkdownDescription: "Subnet mask as prefix length (0-32).",
			},
			function.StringParameter{
				Name:                "format",
				MarkdownDescription: "Required output format: 'dotted-decimal' for standard notation (e.g., '255.255.255.0').",
			},
		},
		Return: function.StringReturn{},
	}
}

func (r NormalizeMaskFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var maskValue types.Number
	var formatValue string

	resp.Error = function.ConcatFuncErrors(req.Arguments.Get(ctx, &maskValue, &formatValue))
	if resp.Error != nil {
		return
	}

	// Handle null or unknown mask value
	if maskValue.IsNull() || maskValue.IsUnknown() {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Mask parameter cannot be null or unknown"))
		return
	}

	// Validate format parameter
	if !ValidMaskFormats[formatValue] {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError(fmt.Sprintf("Invalid format '%s'. Must be '%s'", formatValue, FormatDottedDecimal)))
		return
	}

	// Extract integer value from mask
	maskInt64, _ := maskValue.ValueBigFloat().Int64()

	// Validate mask prefix length
	if err := validateMaskPrefix(maskInt64); err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError(err.Error()))
		return
	}

	// Convert to requested format
	var result string
	switch formatValue {
	case FormatDottedDecimal:
		result = convertPrefixToDottedDecimal(maskInt64)
	default:
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError(fmt.Sprintf("Unsupported format '%s'", formatValue)))
		return
	}

	resp.Error = function.ConcatFuncErrors(resp.Result.Set(ctx, result))
}
