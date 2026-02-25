package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

func TestParseBgpRdRtFunction_Auto(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsParseBgpRdRt_auto(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("format", "auto"),
					resource.TestCheckOutput("as_number", "0"),
					resource.TestCheckOutput("assigned_number", "0"),
					resource.TestCheckOutput("ipv4_address", ""),
				),
			},
		},
	})
}

func TestParseBgpRdRtFunction_AutoUpperCase(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsParseBgpRdRt_autoUpperCase(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("format", "auto"),
					resource.TestCheckOutput("as_number", "0"),
					resource.TestCheckOutput("assigned_number", "0"),
					resource.TestCheckOutput("ipv4_address", ""),
				),
			},
		},
	})
}

func TestParseBgpRdRtFunction_AutoMixedCase(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsParseBgpRdRt_autoMixedCase(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("format", "auto"),
					resource.TestCheckOutput("as_number", "0"),
					resource.TestCheckOutput("assigned_number", "0"),
					resource.TestCheckOutput("ipv4_address", ""),
				),
			},
		},
	})
}

func TestParseBgpRdRtFunction_TwoByteAS(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsParseBgpRdRt_twoByteAS(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("format", "two_byte_as"),
					resource.TestCheckOutput("as_number", "65000"),
					resource.TestCheckOutput("assigned_number", "1001"),
					resource.TestCheckOutput("ipv4_address", ""),
				),
			},
		},
	})
}

func TestParseBgpRdRtFunction_FourByteAS(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsParseBgpRdRt_fourByteAS(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("format", "four_byte_as"),
					resource.TestCheckOutput("as_number", "4200000001"),
					resource.TestCheckOutput("assigned_number", "1003"),
					resource.TestCheckOutput("ipv4_address", ""),
				),
			},
		},
	})
}

func TestParseBgpRdRtFunction_IPv4Address(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsParseBgpRdRt_ipv4Address(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("format", "ipv4_address"),
					resource.TestCheckOutput("as_number", "0"),
					resource.TestCheckOutput("assigned_number", "1002"),
					resource.TestCheckOutput("ipv4_address", "192.168.100.1"),
				),
			},
		},
	})
}

func TestParseBgpRdRtFunction_BoundaryTwoByteAS(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsParseBgpRdRt_boundaryTwoByteAS(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("format", "two_byte_as"),
					resource.TestCheckOutput("as_number", "65535"),
					resource.TestCheckOutput("assigned_number", "100"),
				),
			},
		},
	})
}

func TestParseBgpRdRtFunction_BoundaryFourByteAS(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsParseBgpRdRt_boundaryFourByteAS(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("format", "four_byte_as"),
					resource.TestCheckOutput("as_number", "65536"),
					resource.TestCheckOutput("assigned_number", "100"),
				),
			},
		},
	})
}

func TestParseBgpRdRtFunction_MinTwoByteAS(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsParseBgpRdRt_minTwoByteAS(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("format", "two_byte_as"),
					resource.TestCheckOutput("as_number", "1"),
					resource.TestCheckOutput("assigned_number", "0"),
				),
			},
		},
	})
}

func TestParseBgpRdRtFunction_InvalidNoColon(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccFunctionUtilsParseBgpRdRt_invalidNoColon(),
				ExpectError: regexp.MustCompile(`(?s)invalid BGP RD/RT[\s\S]*format.*65000.*expected.*colon notation`),
			},
		},
	})
}

func TestParseBgpRdRtFunction_InvalidLeftSide(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccFunctionUtilsParseBgpRdRt_invalidLeftSide(),
				ExpectError: regexp.MustCompile(`(?s)invalid AS number[\s\S]*abc[\s\S]*must be a non-negative integer`),
			},
		},
	})
}

func TestParseBgpRdRtFunction_InvalidRightSide(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccFunctionUtilsParseBgpRdRt_invalidRightSide(),
				ExpectError: regexp.MustCompile(`(?s)invalid assigned[\s\S]*number[\s\S]*abc[\s\S]*must be a non-negative integer`),
			},
		},
	})
}

func TestParseBgpRdRtFunction_InvalidIPv4(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccFunctionUtilsParseBgpRdRt_invalidIPv4(),
				ExpectError: regexp.MustCompile(`(?s)invalid IPv4[\s\S]*address.*999\.999\.999\.999.*must be a valid IPv4`),
			},
		},
	})
}

func TestParseBgpRdRtFunction_InvalidZeroAS(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccFunctionUtilsParseBgpRdRt_invalidZeroAS(),
				ExpectError: regexp.MustCompile(`(?s)invalid AS number[\s\S]*0[\s\S]*two-byte AS number must be 1-65535`),
			},
		},
	})
}

func TestParseBgpRdRtFunction_InvalidNegativeAS(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccFunctionUtilsParseBgpRdRt_invalidNegativeAS(),
				ExpectError: regexp.MustCompile(`(?s)invalid AS number[\s\S]*must be a non-negative integer`),
			},
		},
	})
}

func TestParseBgpRdRtFunction_InvalidNegativeAssigned(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccFunctionUtilsParseBgpRdRt_invalidNegativeAssigned(),
				ExpectError: regexp.MustCompile(`(?s)invalid assigned[\s\S]*number[\s\S]*must be a non-negative integer`),
			},
		},
	})
}

func TestParseBgpRdRtFunction_InvalidFourByteAssignedTooLarge(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccFunctionUtilsParseBgpRdRt_invalidFourByteAssignedTooLarge(),
				ExpectError: regexp.MustCompile(`(?s)invalid assigned[\s\S]*number 70000[\s\S]*four-byte AS format assigned number must be[\s\S]*0-65535`),
			},
		},
	})
}

func TestParseBgpRdRtFunction_InvalidIPv4AssignedTooLarge(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccFunctionUtilsParseBgpRdRt_invalidIPv4AssignedTooLarge(),
				ExpectError: regexp.MustCompile(`(?s)invalid assigned[\s\S]*number 70000[\s\S]*IPv4 address format assigned number must be[\s\S]*0-65535`),
			},
		},
	})
}

func TestParseBgpRdRtFunction_InvalidASTooLarge(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccFunctionUtilsParseBgpRdRt_invalidASTooLarge(),
				ExpectError: regexp.MustCompile(`(?s)invalid AS number[\s\S]*4294967296[\s\S]*four-byte AS number must be 65536-4294967295`),
			},
		},
	})
}

func TestParseBgpRdRtFunction_MaxTwoByteAssigned(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsParseBgpRdRt_maxTwoByteAssigned(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("format", "two_byte_as"),
					resource.TestCheckOutput("as_number", "1"),
					resource.TestCheckOutput("assigned_number", "4294967295"),
				),
			},
		},
	})
}

func TestParseBgpRdRtFunction_MaxFourByteAS(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsParseBgpRdRt_maxFourByteAS(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("format", "four_byte_as"),
					resource.TestCheckOutput("as_number", "4294967295"),
					resource.TestCheckOutput("assigned_number", "65535"),
				),
			},
		},
	})
}

// Test configuration functions

func testAccFunctionUtilsParseBgpRdRt_auto() string {
	return `
locals {
  result = provider::utils::parse_bgp_rd_rt("auto")
}

output "format" {
  value = local.result.format
}

output "as_number" {
  value = local.result.as_number
}

output "assigned_number" {
  value = local.result.assigned_number
}

output "ipv4_address" {
  value = local.result.ipv4_address
}
`
}

func testAccFunctionUtilsParseBgpRdRt_autoUpperCase() string {
	return `
locals {
  result = provider::utils::parse_bgp_rd_rt("AUTO")
}

output "format" {
  value = local.result.format
}

output "as_number" {
  value = local.result.as_number
}

output "assigned_number" {
  value = local.result.assigned_number
}

output "ipv4_address" {
  value = local.result.ipv4_address
}
`
}

func testAccFunctionUtilsParseBgpRdRt_autoMixedCase() string {
	return `
locals {
  result = provider::utils::parse_bgp_rd_rt("Auto")
}

output "format" {
  value = local.result.format
}

output "as_number" {
  value = local.result.as_number
}

output "assigned_number" {
  value = local.result.assigned_number
}

output "ipv4_address" {
  value = local.result.ipv4_address
}
`
}

func testAccFunctionUtilsParseBgpRdRt_twoByteAS() string {
	return `
locals {
  result = provider::utils::parse_bgp_rd_rt("65000:1001")
}

output "format" {
  value = local.result.format
}

output "as_number" {
  value = local.result.as_number
}

output "assigned_number" {
  value = local.result.assigned_number
}

output "ipv4_address" {
  value = local.result.ipv4_address
}
`
}

func testAccFunctionUtilsParseBgpRdRt_fourByteAS() string {
	return `
locals {
  result = provider::utils::parse_bgp_rd_rt("4200000001:1003")
}

output "format" {
  value = local.result.format
}

output "as_number" {
  value = local.result.as_number
}

output "assigned_number" {
  value = local.result.assigned_number
}

output "ipv4_address" {
  value = local.result.ipv4_address
}
`
}

func testAccFunctionUtilsParseBgpRdRt_ipv4Address() string {
	return `
locals {
  result = provider::utils::parse_bgp_rd_rt("192.168.100.1:1002")
}

output "format" {
  value = local.result.format
}

output "as_number" {
  value = local.result.as_number
}

output "assigned_number" {
  value = local.result.assigned_number
}

output "ipv4_address" {
  value = local.result.ipv4_address
}
`
}

func testAccFunctionUtilsParseBgpRdRt_boundaryTwoByteAS() string {
	return `
locals {
  result = provider::utils::parse_bgp_rd_rt("65535:100")
}

output "format" {
  value = local.result.format
}

output "as_number" {
  value = local.result.as_number
}

output "assigned_number" {
  value = local.result.assigned_number
}
`
}

func testAccFunctionUtilsParseBgpRdRt_boundaryFourByteAS() string {
	return `
locals {
  result = provider::utils::parse_bgp_rd_rt("65536:100")
}

output "format" {
  value = local.result.format
}

output "as_number" {
  value = local.result.as_number
}

output "assigned_number" {
  value = local.result.assigned_number
}
`
}

func testAccFunctionUtilsParseBgpRdRt_minTwoByteAS() string {
	return `
locals {
  result = provider::utils::parse_bgp_rd_rt("1:0")
}

output "format" {
  value = local.result.format
}

output "as_number" {
  value = local.result.as_number
}

output "assigned_number" {
  value = local.result.assigned_number
}
`
}

func testAccFunctionUtilsParseBgpRdRt_invalidNoColon() string {
	return `
output "invalid" {
  value = provider::utils::parse_bgp_rd_rt("65000")
}
`
}

func testAccFunctionUtilsParseBgpRdRt_invalidLeftSide() string {
	return `
output "invalid" {
  value = provider::utils::parse_bgp_rd_rt("abc:100")
}
`
}

func testAccFunctionUtilsParseBgpRdRt_invalidRightSide() string {
	return `
output "invalid" {
  value = provider::utils::parse_bgp_rd_rt("65000:abc")
}
`
}

func testAccFunctionUtilsParseBgpRdRt_invalidIPv4() string {
	return `
output "invalid" {
  value = provider::utils::parse_bgp_rd_rt("999.999.999.999:100")
}
`
}

func testAccFunctionUtilsParseBgpRdRt_invalidZeroAS() string {
	return `
output "invalid" {
  value = provider::utils::parse_bgp_rd_rt("0:100")
}
`
}

func testAccFunctionUtilsParseBgpRdRt_invalidNegativeAS() string {
	return `
output "invalid" {
  value = provider::utils::parse_bgp_rd_rt("-1:100")
}
`
}

func testAccFunctionUtilsParseBgpRdRt_invalidNegativeAssigned() string {
	return `
output "invalid" {
  value = provider::utils::parse_bgp_rd_rt("65000:-1")
}
`
}

func testAccFunctionUtilsParseBgpRdRt_invalidFourByteAssignedTooLarge() string {
	return `
output "invalid" {
  value = provider::utils::parse_bgp_rd_rt("100000:70000")
}
`
}

func testAccFunctionUtilsParseBgpRdRt_invalidIPv4AssignedTooLarge() string {
	return `
output "invalid" {
  value = provider::utils::parse_bgp_rd_rt("10.0.0.1:70000")
}
`
}

func testAccFunctionUtilsParseBgpRdRt_invalidASTooLarge() string {
	return `
output "invalid" {
  value = provider::utils::parse_bgp_rd_rt("4294967296:100")
}
`
}

func testAccFunctionUtilsParseBgpRdRt_maxTwoByteAssigned() string {
	return `
locals {
  result = provider::utils::parse_bgp_rd_rt("1:4294967295")
}

output "format" {
  value = local.result.format
}

output "as_number" {
  value = local.result.as_number
}

output "assigned_number" {
  value = local.result.assigned_number
}
`
}

func testAccFunctionUtilsParseBgpRdRt_maxFourByteAS() string {
	return `
locals {
  result = provider::utils::parse_bgp_rd_rt("4294967295:65535")
}

output "format" {
  value = local.result.format
}

output "as_number" {
  value = local.result.as_number
}

output "assigned_number" {
  value = local.result.assigned_number
}
`
}
