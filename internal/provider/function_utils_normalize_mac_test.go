package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

func TestNormalizeMacFunction_Dotted(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsNormalizeMac_dotted(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("from_colon", "0011.2233.4455"),
					resource.TestCheckOutput("from_dash", "0011.2233.4455"),
					resource.TestCheckOutput("from_dotted", "0011.2233.4455"),
				),
			},
		},
	})
}

func TestNormalizeMacFunction_Colon(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsNormalizeMac_colon(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("from_dotted", "00:11:22:33:44:55"),
					resource.TestCheckOutput("from_dash", "00:11:22:33:44:55"),
					resource.TestCheckOutput("from_colon", "00:11:22:33:44:55"),
				),
			},
		},
	})
}

func TestNormalizeMacFunction_Dash(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsNormalizeMac_dash(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("from_dotted", "00-11-22-33-44-55"),
					resource.TestCheckOutput("from_colon", "00-11-22-33-44-55"),
					resource.TestCheckOutput("from_dash", "00-11-22-33-44-55"),
				),
			},
		},
	})
}

func TestNormalizeMacFunction_MixedCase(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsNormalizeMac_mixedCase(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("uppercase_to_dotted", "aabb.ccdd.eeff"),
					resource.TestCheckOutput("uppercase_to_colon", "aa:bb:cc:dd:ee:ff"),
					resource.TestCheckOutput("mixed_to_dash", "aa-bb-cc-dd-ee-ff"),
				),
			},
		},
	})
}

func TestNormalizeMacFunction_InvalidFormat(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccFunctionUtilsNormalizeMac_invalidFormat(),
				ExpectError: regexp.MustCompile(`(?s)Invalid format.*'hex'`),
			},
		},
	})
}

func TestNormalizeMacFunction_TooShort(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccFunctionUtilsNormalizeMac_tooShort(),
				ExpectError: regexp.MustCompile(`(?s)invalid MAC address[\s\S]*00:11:22:33:44`),
			},
		},
	})
}

func TestNormalizeMacFunction_TooLong(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccFunctionUtilsNormalizeMac_tooLong(),
				ExpectError: regexp.MustCompile(`(?s)invalid MAC address[\s\S]*00:11:22:33:44:55:66`),
			},
		},
	})
}

func TestNormalizeMacFunction_InvalidChars(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccFunctionUtilsNormalizeMac_invalidChars(),
				ExpectError: regexp.MustCompile(`(?s)invalid MAC address[\s\S]*GG:HH:II:JJ:KK:LL`),
			},
		},
	})
}

// Test configuration functions

func testAccFunctionUtilsNormalizeMac_dotted() string {
	return `
output "from_colon" {
  value = provider::utils::normalize_mac("00:11:22:33:44:55", "dotted")
}

output "from_dash" {
  value = provider::utils::normalize_mac("00-11-22-33-44-55", "dotted")
}

output "from_dotted" {
  value = provider::utils::normalize_mac("0011.2233.4455", "dotted")
}
`
}

func testAccFunctionUtilsNormalizeMac_colon() string {
	return `
output "from_dotted" {
  value = provider::utils::normalize_mac("0011.2233.4455", "colon")
}

output "from_dash" {
  value = provider::utils::normalize_mac("00-11-22-33-44-55", "colon")
}

output "from_colon" {
  value = provider::utils::normalize_mac("00:11:22:33:44:55", "colon")
}
`
}

func testAccFunctionUtilsNormalizeMac_dash() string {
	return `
output "from_dotted" {
  value = provider::utils::normalize_mac("0011.2233.4455", "dash")
}

output "from_colon" {
  value = provider::utils::normalize_mac("00:11:22:33:44:55", "dash")
}

output "from_dash" {
  value = provider::utils::normalize_mac("00-11-22-33-44-55", "dash")
}
`
}

func testAccFunctionUtilsNormalizeMac_mixedCase() string {
	return `
output "uppercase_to_dotted" {
  value = provider::utils::normalize_mac("AA:BB:CC:DD:EE:FF", "dotted")
}

output "uppercase_to_colon" {
  value = provider::utils::normalize_mac("AABB.CCDD.EEFF", "colon")
}

output "mixed_to_dash" {
  value = provider::utils::normalize_mac("Aa-Bb-Cc-Dd-Ee-Ff", "dash")
}
`
}

func testAccFunctionUtilsNormalizeMac_invalidFormat() string {
	return `
output "invalid" {
  value = provider::utils::normalize_mac("00:11:22:33:44:55", "hex")
}
`
}

func testAccFunctionUtilsNormalizeMac_tooShort() string {
	return `
output "too_short" {
  value = provider::utils::normalize_mac("00:11:22:33:44", "dotted")
}
`
}

func testAccFunctionUtilsNormalizeMac_tooLong() string {
	return `
output "too_long" {
  value = provider::utils::normalize_mac("00:11:22:33:44:55:66", "dotted")
}
`
}

func testAccFunctionUtilsNormalizeMac_invalidChars() string {
	return `
output "invalid_chars" {
  value = provider::utils::normalize_mac("GG:HH:II:JJ:KK:LL", "dotted")
}
`
}
