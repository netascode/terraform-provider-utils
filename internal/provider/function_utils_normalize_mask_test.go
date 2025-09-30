package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

func TestNormalizeMaskFunction_CommonMasks(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsNormalizeMask_commonMasks(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test_24", "255.255.255.0"),
					resource.TestCheckOutput("test_32", "255.255.255.255"),
					resource.TestCheckOutput("test_16", "255.255.0.0"),
					resource.TestCheckOutput("test_8", "255.0.0.0"),
					resource.TestCheckOutput("test_0", "0.0.0.0"),
				),
			},
		},
	})
}

func TestNormalizeMaskFunction_AllValidMasks(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsNormalizeMask_allValid(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test_1", "128.0.0.0"),
					resource.TestCheckOutput("test_15", "255.254.0.0"),
					resource.TestCheckOutput("test_23", "255.255.254.0"),
					resource.TestCheckOutput("test_25", "255.255.255.128"),
					resource.TestCheckOutput("test_30", "255.255.255.252"),
					resource.TestCheckOutput("test_31", "255.255.255.254"),
				),
			},
		},
	})
}

func TestNormalizeMaskFunction_OutOfRangeLow(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccFunctionUtilsNormalizeMask_outOfRangeLow(),
				ExpectError: regexp.MustCompile(`mask prefix length[\s\n]-1[\s\n]is out of valid range.*0-32`),
			},
		},
	})
}

func TestNormalizeMaskFunction_OutOfRangeHigh(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccFunctionUtilsNormalizeMask_outOfRangeHigh(),
				ExpectError: regexp.MustCompile(`mask prefix length[\s\n]33[\s\n]is out of valid range.*0-32`),
			},
		},
	})
}

func TestNormalizeMaskFunction_OutOfRangeLarge(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccFunctionUtilsNormalizeMask_outOfRangeLarge(),
				ExpectError: regexp.MustCompile(`mask prefix length[\s\n]65[\s\n]is out of valid range.*0-32`),
			},
		},
	})
}

func TestNormalizeMaskFunction_InvalidFormat(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccFunctionUtilsNormalizeMask_invalidFormat(),
				ExpectError: regexp.MustCompile(`(?s)Invalid format.*binary.*Must be.*dotted-decimal`),
			},
		},
	})
}

func TestNormalizeMaskFunction_NullInput(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccFunctionUtilsNormalizeMask_nullInput(),
				ExpectError: regexp.MustCompile(`argument must not be null`),
			},
		},
	})
}

// Test configuration functions

func testAccFunctionUtilsNormalizeMask_commonMasks() string {
	return `
	output "test_24" {
		value = provider::utils::normalize_mask(24, "dotted-decimal")
	}

	output "test_32" {
		value = provider::utils::normalize_mask(32, "dotted-decimal")
	}

	output "test_16" {
		value = provider::utils::normalize_mask(16, "dotted-decimal")
	}

	output "test_8" {
		value = provider::utils::normalize_mask(8, "dotted-decimal")
	}

	output "test_0" {
		value = provider::utils::normalize_mask(0, "dotted-decimal")
	}
	`
}

func testAccFunctionUtilsNormalizeMask_allValid() string {
	return `
	output "test_1" {
		value = provider::utils::normalize_mask(1, "dotted-decimal")
	}

	output "test_15" {
		value = provider::utils::normalize_mask(15, "dotted-decimal")
	}

	output "test_23" {
		value = provider::utils::normalize_mask(23, "dotted-decimal")
	}

	output "test_25" {
		value = provider::utils::normalize_mask(25, "dotted-decimal")
	}

	output "test_30" {
		value = provider::utils::normalize_mask(30, "dotted-decimal")
	}

	output "test_31" {
		value = provider::utils::normalize_mask(31, "dotted-decimal")
	}
	`
}

func testAccFunctionUtilsNormalizeMask_outOfRangeLow() string {
	return `
	output "test" {
		value = provider::utils::normalize_mask(-1, "dotted-decimal")
	}
	`
}

func testAccFunctionUtilsNormalizeMask_outOfRangeHigh() string {
	return `
	output "test" {
		value = provider::utils::normalize_mask(33, "dotted-decimal")
	}
	`
}

func testAccFunctionUtilsNormalizeMask_outOfRangeLarge() string {
	return `
	output "test" {
		value = provider::utils::normalize_mask(65, "dotted-decimal")
	}
	`
}

func testAccFunctionUtilsNormalizeMask_invalidFormat() string {
	return `
	output "test" {
		value = provider::utils::normalize_mask(24, "binary")
	}
	`
}

func testAccFunctionUtilsNormalizeMask_nullInput() string {
	return `
	output "test" {
		value = provider::utils::normalize_mask(null, "dotted-decimal")
	}
	`
}
