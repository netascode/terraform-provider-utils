package provider

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/function"
)

// Format constants for MAC address normalization
const (
	FormatMacDotted = "dotted" // xxxx.xxxx.xxxx (Cisco)
	FormatMacColon  = "colon"  // xx:xx:xx:xx:xx:xx (IEEE 802)
	FormatMacDash   = "dash"   // xx-xx-xx-xx-xx-xx
)

// ValidMacFormats defines valid format values
var ValidMacFormats = map[string]bool{
	FormatMacDotted: true,
	FormatMacColon:  true,
	FormatMacDash:   true,
}

var _ function.Function = NormalizeMacFunction{}

// cleanMacAddress removes common delimiters from MAC address and validates format
func cleanMacAddress(mac string) (string, error) {
	// Remove common delimiters: colons, dashes, dots
	cleaned := strings.ReplaceAll(mac, ":", "")
	cleaned = strings.ReplaceAll(cleaned, "-", "")
	cleaned = strings.ReplaceAll(cleaned, ".", "")
	cleaned = strings.ToLower(cleaned)

	// Validate that cleaned MAC is exactly 12 hex characters
	matched, _ := regexp.MatchString("^[0-9a-f]{12}$", cleaned)
	if !matched {
		return "", fmt.Errorf("invalid MAC address format: %s (must be 12 hexadecimal characters after removing delimiters)", mac)
	}

	return cleaned, nil
}

// formatMacAddress formats a cleaned MAC address string to the specified format
func formatMacAddress(cleaned, format string) string {
	switch format {
	case FormatMacDotted:
		// xxxx.xxxx.xxxx
		return fmt.Sprintf("%s.%s.%s", cleaned[0:4], cleaned[4:8], cleaned[8:12])
	case FormatMacColon:
		// xx:xx:xx:xx:xx:xx
		parts := make([]string, 6)
		for i := 0; i < 6; i++ {
			parts[i] = cleaned[i*2 : i*2+2]
		}
		return strings.Join(parts, ":")
	case FormatMacDash:
		// xx-xx-xx-xx-xx-xx
		parts := make([]string, 6)
		for i := 0; i < 6; i++ {
			parts[i] = cleaned[i*2 : i*2+2]
		}
		return strings.Join(parts, "-")
	default:
		return cleaned
	}
}

func NewNormalizeMacFunction() function.Function {
	return &NormalizeMacFunction{}
}

type NormalizeMacFunction struct{}

func (r NormalizeMacFunction) Metadata(_ context.Context, req function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "normalize_mac"
}

func (r NormalizeMacFunction) Definition(_ context.Context, _ function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary:             "Normalize MAC address to specified format",
		MarkdownDescription: "Takes a MAC address in any common format (colon-separated, dash-separated, or dotted) and a format parameter. Returns the MAC address in the specified format. Supports 'dotted' (Cisco xxxx.xxxx.xxxx), 'colon' (IEEE 802 xx:xx:xx:xx:xx:xx), and 'dash' (xx-xx-xx-xx-xx-xx) formats.",
		Parameters: []function.Parameter{
			function.StringParameter{
				Name:                "mac",
				MarkdownDescription: "MAC address in any common format (e.g., '00:11:22:33:44:55', '00-11-22-33-44-55', or '0011.2233.4455').",
			},
			function.StringParameter{
				Name:                "format",
				MarkdownDescription: "Required output format: 'dotted' for Cisco notation (xxxx.xxxx.xxxx), 'colon' for IEEE 802 standard (xx:xx:xx:xx:xx:xx), or 'dash' for dash-separated (xx-xx-xx-xx-xx-xx).",
			},
		},
		Return: function.StringReturn{},
	}
}

func (r NormalizeMacFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var macValue string
	var formatValue string

	resp.Error = function.ConcatFuncErrors(req.Arguments.Get(ctx, &macValue, &formatValue))
	if resp.Error != nil {
		return
	}

	// Validate format parameter
	if !ValidMacFormats[formatValue] {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError(fmt.Sprintf("Invalid format '%s'. Must be one of: 'dotted', 'colon', 'dash'", formatValue)))
		return
	}

	// Clean and validate MAC address
	cleaned, err := cleanMacAddress(macValue)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError(err.Error()))
		return
	}

	// Format to requested output
	result := formatMacAddress(cleaned, formatValue)

	resp.Error = function.ConcatFuncErrors(resp.Result.Set(ctx, result))
}
