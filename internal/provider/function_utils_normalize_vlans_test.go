package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

func TestNormalizeVlansFunction_Known(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsNormalizeVlans_basic(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test", "1-2,5,10-30,40-50"),
				),
			},
		},
	})
}

func TestNormalizeVlansFunction_SingleVlans(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsNormalizeVlans_singleVlans(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test", "1,5,10,20"),
				),
			},
		},
	})
}

func TestNormalizeVlansFunction_RangesOnly(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsNormalizeVlans_rangesOnly(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test", "10-20,30-40"),
				),
			},
		},
	})
}

func TestNormalizeVlansFunction_OverlappingRanges(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsNormalizeVlans_overlappingRanges(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test", "10-25"),
				),
			},
		},
	})
}

func TestNormalizeVlansFunction_AdjacentRanges(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsNormalizeVlans_adjacentRanges(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test", "10-30"),
				),
			},
		},
	})
}

func TestNormalizeVlansFunction_EmptyInput(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsNormalizeVlans_empty(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test", ""),
				),
			},
		},
	})
}

func TestNormalizeVlansFunction_SingleRange(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsNormalizeVlans_singleRange(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test", "100"),
				),
			},
		},
	})
}

func TestNormalizeVlansFunction_DuplicateIds(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsNormalizeVlans_duplicateIds(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test", "1,5,10"),
				),
			},
		},
	})
}

func TestNormalizeVlansFunction_InvalidRange(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccFunctionUtilsNormalizeVlans_invalidRange(),
				ExpectError: regexp.MustCompile(`(?s)VLAN range.*20.*cannot be greater than.*10`),
			},
		},
	})
}

func TestNormalizeVlansFunction_OutOfRangeVlan(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccFunctionUtilsNormalizeVlans_outOfRange(),
				ExpectError: regexp.MustCompile(`(?s)VLAN ID 5000.*out of valid range.*1-4094`),
			},
		},
	})
}

func TestNormalizeVlansFunction_NullFields(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsNormalizeVlans_nullFields(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test", ""),
				),
			},
		},
	})
}

// Test configuration functions

func testAccFunctionUtilsNormalizeVlans_basic() string {
	return `
	output "test" {
		value = provider::utils::normalize_vlans({
			ids = [1, 2, 5]
			ranges = [
				{ from = 10, to = 20 },
				{ from = 21, to = 30 },
				{ from = 40, to = 50 }
			]
		}, "string")
	}
	`
}

func testAccFunctionUtilsNormalizeVlans_singleVlans() string {
	return `
	output "test" {
		value = provider::utils::normalize_vlans({
			ids = [1, 5, 10, 20]
		}, "string")
	}
	`
}

func testAccFunctionUtilsNormalizeVlans_rangesOnly() string {
	return `
	output "test" {
		value = provider::utils::normalize_vlans({
			ranges = [
				{ from = 10, to = 20 },
				{ from = 30, to = 40 }
			]
		}, "string")
	}
	`
}

func testAccFunctionUtilsNormalizeVlans_overlappingRanges() string {
	return `
	output "test" {
		value = provider::utils::normalize_vlans({
			ranges = [
				{ from = 10, to = 20 },
				{ from = 15, to = 25 }
			]
		}, "string")
	}
	`
}

func testAccFunctionUtilsNormalizeVlans_adjacentRanges() string {
	return `
	output "test" {
		value = provider::utils::normalize_vlans({
			ranges = [
				{ from = 10, to = 20 },
				{ from = 21, to = 30 }
			]
		}, "string")
	}
	`
}

func testAccFunctionUtilsNormalizeVlans_empty() string {
	return `
	output "test" {
		value = provider::utils::normalize_vlans({}, "string")
	}
	`
}

func testAccFunctionUtilsNormalizeVlans_singleRange() string {
	return `
	output "test" {
		value = provider::utils::normalize_vlans({
			ranges = [
				{ from = 100, to = 100 }
			]
		}, "string")
	}
	`
}

func testAccFunctionUtilsNormalizeVlans_duplicateIds() string {
	return `
	output "test" {
		value = provider::utils::normalize_vlans({
			ids = [1, 5, 10, 1, 5]
		}, "string")
	}
	`
}

func testAccFunctionUtilsNormalizeVlans_invalidRange() string {
	return `
	output "test" {
		value = provider::utils::normalize_vlans({
			ranges = [
				{ from = 20, to = 10 }
			]
		}, "string")
	}
	`
}

func testAccFunctionUtilsNormalizeVlans_outOfRange() string {
	return `
	output "test" {
		value = provider::utils::normalize_vlans({
			ids = [5000]
		}, "string")
	}
	`
}

func testAccFunctionUtilsNormalizeVlans_nullFields() string {
	return `
	output "test" {
		value = provider::utils::normalize_vlans({
			ids = null
			ranges = null
		}, "string")
	}
	`
}

// Test functions for list format
func TestNormalizeVlansFunction_ListFormat_Basic(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsNormalizeVlans_listFormat_basic(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test_length", "35"), // 3 ids + 32 from ranges (10-30: 21, 40-50: 11)
					resource.TestCheckOutput("test_contains_1", "true"),
					resource.TestCheckOutput("test_contains_2", "true"),
					resource.TestCheckOutput("test_contains_5", "true"),
					resource.TestCheckOutput("test_contains_10", "true"),
					resource.TestCheckOutput("test_contains_30", "true"),
					resource.TestCheckOutput("test_contains_50", "true"),
				),
			},
		},
	})
}

func TestNormalizeVlansFunction_ListFormat_SingleVlans(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsNormalizeVlans_listFormat_singleVlans(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test_length", "4"), // [1, 5, 10, 20]
					resource.TestCheckOutput("test_first", "1"),
					resource.TestCheckOutput("test_second", "5"),
					resource.TestCheckOutput("test_third", "10"),
					resource.TestCheckOutput("test_fourth", "20"),
				),
			},
		},
	})
}

func TestNormalizeVlansFunction_ListFormat_EmptyInput(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsNormalizeVlans_listFormat_empty(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test_length", "0"), // Empty list
				),
			},
		},
	})
}

func TestNormalizeVlansFunction_StringFormat_Explicit(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsNormalizeVlans_stringFormat_explicit(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test", "1-2,5,10-30"),
				),
			},
		},
	})
}

func TestNormalizeVlansFunction_InvalidFormat(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccFunctionUtilsNormalizeVlans_invalidFormat(),
				ExpectError: regexp.MustCompile(`(?s)Invalid format.*json.*Must be.*string.*or.*list`),
			},
		},
	})
}

// Test configuration functions for new format parameter tests

func testAccFunctionUtilsNormalizeVlans_listFormat_basic() string {
	return `
	locals {
		vlan_result = provider::utils::normalize_vlans({
			ids = [1, 2, 5]
			ranges = [
				{ from = 10, to = 30 },
				{ from = 40, to = 50 }
			]
		}, "list")
	}

	output "test_length" {
		value = length(local.vlan_result)
	}

	output "test_contains_1" {
		value = contains(local.vlan_result, 1)
	}

	output "test_contains_2" {
		value = contains(local.vlan_result, 2)
	}

	output "test_contains_5" {
		value = contains(local.vlan_result, 5)
	}

	output "test_contains_10" {
		value = contains(local.vlan_result, 10)
	}

	output "test_contains_30" {
		value = contains(local.vlan_result, 30)
	}

	output "test_contains_50" {
		value = contains(local.vlan_result, 50)
	}
	`
}

func testAccFunctionUtilsNormalizeVlans_listFormat_singleVlans() string {
	return `
	locals {
		vlan_result = provider::utils::normalize_vlans({
			ids = [1, 5, 10, 20]
		}, "list")
	}

	output "test_length" {
		value = length(local.vlan_result)
	}

	output "test_first" {
		value = local.vlan_result[0]
	}

	output "test_second" {
		value = local.vlan_result[1]
	}

	output "test_third" {
		value = local.vlan_result[2]
	}

	output "test_fourth" {
		value = local.vlan_result[3]
	}
	`
}

func testAccFunctionUtilsNormalizeVlans_listFormat_empty() string {
	return `
	locals {
		vlan_result = provider::utils::normalize_vlans({}, "list")
	}

	output "test_length" {
		value = length(local.vlan_result)
	}
	`
}

func testAccFunctionUtilsNormalizeVlans_stringFormat_explicit() string {
	return `
	output "test" {
		value = provider::utils::normalize_vlans({
			ids = [1, 2, 5]
			ranges = [
				{ from = 10, to = 30 }
			]
		}, "string")
	}
	`
}

func testAccFunctionUtilsNormalizeVlans_invalidFormat() string {
	return `
	output "test" {
		value = provider::utils::normalize_vlans({
			ids = [1, 2, 5]
		}, "json")
	}
	`
}