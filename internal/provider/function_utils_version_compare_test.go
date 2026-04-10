package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

func TestVersionCompareFunction_Equal(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
					output "test_equal_simple" {
						value = provider::utils::version_compare("1.2.3", "1.2.3")
					}
					output "test_equal_large" {
						value = provider::utils::version_compare("25.2.2", "25.2.2")
					}
					output "test_equal_with_v_prefix" {
						value = provider::utils::version_compare("v1.2.3", "v1.2.3")
					}
					output "test_equal_mixed_prefix" {
						value = provider::utils::version_compare("v1.2.3", "1.2.3")
					}
				`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test_equal_simple", "0"),
					resource.TestCheckOutput("test_equal_large", "0"),
					resource.TestCheckOutput("test_equal_with_v_prefix", "0"),
					resource.TestCheckOutput("test_equal_mixed_prefix", "0"),
				),
			},
		},
	})
}

func TestVersionCompareFunction_GreaterThan(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
					output "test_major_greater" {
						value = provider::utils::version_compare("2.0.0", "1.0.0")
					}
					output "test_minor_greater" {
						value = provider::utils::version_compare("1.3.0", "1.2.0")
					}
					output "test_patch_greater" {
						value = provider::utils::version_compare("1.2.4", "1.2.3")
					}
					output "test_large_version" {
						value = provider::utils::version_compare("25.3.0", "25.2.2")
					}
					output "test_positive_number" {
						value = provider::utils::version_compare("2.0.0", "1.0.0") > 0
					}
				`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test_major_greater", "1"),
					resource.TestCheckOutput("test_minor_greater", "1"),
					resource.TestCheckOutput("test_patch_greater", "1"),
					resource.TestCheckOutput("test_large_version", "1"),
					resource.TestCheckOutput("test_positive_number", "true"),
				),
			},
		},
	})
}

func TestVersionCompareFunction_LessThan(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
					output "test_major_less" {
						value = provider::utils::version_compare("1.0.0", "2.0.0")
					}
					output "test_minor_less" {
						value = provider::utils::version_compare("1.2.0", "1.3.0")
					}
					output "test_patch_less" {
						value = provider::utils::version_compare("1.2.3", "1.2.4")
					}
					output "test_large_version" {
						value = provider::utils::version_compare("24.4.0", "25.2.2")
					}
					output "test_negative_number" {
						value = provider::utils::version_compare("1.0.0", "2.0.0") < 0
					}
				`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test_major_less", "-1"),
					resource.TestCheckOutput("test_minor_less", "-1"),
					resource.TestCheckOutput("test_patch_less", "-1"),
					resource.TestCheckOutput("test_large_version", "-1"),
					resource.TestCheckOutput("test_negative_number", "true"),
				),
			},
		},
	})
}

func TestVersionCompareFunction_Prerelease(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
					output "test_prerelease_less_than_release" {
						value = provider::utils::version_compare("1.2.3-alpha", "1.2.3")
					}
					output "test_prerelease_comparison" {
						value = provider::utils::version_compare("1.2.3-beta", "1.2.3-alpha")
					}
					output "test_with_metadata" {
						value = provider::utils::version_compare("1.2.3+build.1", "1.2.3+build.2")
					}
				`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test_prerelease_less_than_release", "-1"),
					resource.TestCheckOutput("test_prerelease_comparison", "1"),
					resource.TestCheckOutput("test_with_metadata", "0"), // metadata is ignored in comparison
				),
			},
		},
	})
}

func TestVersionCompareFunction_InvalidVersion(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
					output "test_invalid_v1" {
						value = provider::utils::version_compare("invalid", "1.2.3")
					}
				`,
				ExpectError: regexp.MustCompile(`Invalid version[\s\S]*string v1[\s\S]*invalid`),
			},
		},
	})
}

func TestVersionCompareFunction_InvalidVersion2(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
					output "test_invalid_v2" {
						value = provider::utils::version_compare("1.2.3", "not-a-version")
					}
				`,
				ExpectError: regexp.MustCompile(`Invalid version[\s\S]*string v2[\s\S]*not-a-version`),
			},
		},
	})
}

func TestVersionCompareFunction_RealWorldScenario(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
					# Simulating the use case from the user's example
					locals {
						v244_version = "24.4.1"
						v252_version = "25.2.2"
					}
					
					output "test_version_check_for_feature" {
						value = provider::utils::version_compare(local.v244_version, local.v252_version) >= 0
					}
					
					output "test_opposite_check" {
						value = provider::utils::version_compare(local.v252_version, local.v244_version) >= 0
					}
				`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test_version_check_for_feature", "false"),
					resource.TestCheckOutput("test_opposite_check", "true"),
				),
			},
		},
	})
}
