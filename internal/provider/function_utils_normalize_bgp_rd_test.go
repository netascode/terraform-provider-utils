// Copyright © 2022 Cisco Systems, Inc. and its affiliates.
// All rights reserved.
//
// Licensed under the Mozilla Public License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://mozilla.org/MPL/2.0/
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

func TestNormalizeBgpRdFunction_Auto(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsNormalizeBgpRd_auto(),
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

func TestNormalizeBgpRdFunction_AutoUpperCase(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsNormalizeBgpRd_autoUpperCase(),
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

func TestNormalizeBgpRdFunction_AutoMixedCase(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsNormalizeBgpRd_autoMixedCase(),
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

func TestNormalizeBgpRdFunction_TwoByteAS(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsNormalizeBgpRd_twoByteAS(),
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

func TestNormalizeBgpRdFunction_FourByteAS(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsNormalizeBgpRd_fourByteAS(),
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

func TestNormalizeBgpRdFunction_IPv4Address(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsNormalizeBgpRd_ipv4Address(),
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

func TestNormalizeBgpRdFunction_BoundaryTwoByteAS(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsNormalizeBgpRd_boundaryTwoByteAS(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("format", "two_byte_as"),
					resource.TestCheckOutput("as_number", "65535"),
					resource.TestCheckOutput("assigned_number", "100"),
				),
			},
		},
	})
}

func TestNormalizeBgpRdFunction_BoundaryFourByteAS(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsNormalizeBgpRd_boundaryFourByteAS(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("format", "four_byte_as"),
					resource.TestCheckOutput("as_number", "65536"),
					resource.TestCheckOutput("assigned_number", "100"),
				),
			},
		},
	})
}

func TestNormalizeBgpRdFunction_MinTwoByteAS(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsNormalizeBgpRd_minTwoByteAS(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("format", "two_byte_as"),
					resource.TestCheckOutput("as_number", "1"),
					resource.TestCheckOutput("assigned_number", "0"),
				),
			},
		},
	})
}

func TestNormalizeBgpRdFunction_InvalidNoColon(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccFunctionUtilsNormalizeBgpRd_invalidNoColon(),
				ExpectError: regexp.MustCompile(`(?s)invalid BGP RD[\s\S]*format.*65000.*expected.*colon notation`),
			},
		},
	})
}

func TestNormalizeBgpRdFunction_InvalidLeftSide(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccFunctionUtilsNormalizeBgpRd_invalidLeftSide(),
				ExpectError: regexp.MustCompile(`(?s)invalid AS[\s\S]*number[\s\S]*abc[\s\S]*must be a non-negative integer`),
			},
		},
	})
}

func TestNormalizeBgpRdFunction_InvalidRightSide(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccFunctionUtilsNormalizeBgpRd_invalidRightSide(),
				ExpectError: regexp.MustCompile(`(?s)invalid assigned[\s\S]*number[\s\S]*abc[\s\S]*must be a non-negative integer`),
			},
		},
	})
}

func TestNormalizeBgpRdFunction_InvalidIPv4(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccFunctionUtilsNormalizeBgpRd_invalidIPv4(),
				ExpectError: regexp.MustCompile(`(?s)invalid IPv4[\s\S]*address.*999\.999\.999\.999.*must be a valid IPv4`),
			},
		},
	})
}

func TestNormalizeBgpRdFunction_InvalidZeroAS(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccFunctionUtilsNormalizeBgpRd_invalidZeroAS(),
				ExpectError: regexp.MustCompile(`(?s)invalid AS[\s\S]*number[\s\S]*0[\s\S]*two-byte AS number must be 1-65535`),
			},
		},
	})
}

func TestNormalizeBgpRdFunction_InvalidNegativeAS(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccFunctionUtilsNormalizeBgpRd_invalidNegativeAS(),
				ExpectError: regexp.MustCompile(`(?s)invalid AS[\s\S]*number[\s\S]*must be a non-negative integer`),
			},
		},
	})
}

func TestNormalizeBgpRdFunction_InvalidNegativeAssigned(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccFunctionUtilsNormalizeBgpRd_invalidNegativeAssigned(),
				ExpectError: regexp.MustCompile(`(?s)invalid assigned[\s\S]*number[\s\S]*must be a non-negative integer`),
			},
		},
	})
}

func TestNormalizeBgpRdFunction_InvalidFourByteAssignedTooLarge(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccFunctionUtilsNormalizeBgpRd_invalidFourByteAssignedTooLarge(),
				ExpectError: regexp.MustCompile(`(?s)invalid assigned[\s\S]*number 70000[\s\S]*four-byte AS format assigned number must be[\s\S]*0-65535`),
			},
		},
	})
}

func TestNormalizeBgpRdFunction_InvalidIPv4AssignedTooLarge(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccFunctionUtilsNormalizeBgpRd_invalidIPv4AssignedTooLarge(),
				ExpectError: regexp.MustCompile(`(?s)invalid assigned[\s\S]*number 70000[\s\S]*IPv4 address format assigned number must be[\s\S]*0-65535`),
			},
		},
	})
}

func TestNormalizeBgpRdFunction_InvalidASTooLarge(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccFunctionUtilsNormalizeBgpRd_invalidASTooLarge(),
				ExpectError: regexp.MustCompile(`(?s)invalid AS[\s\S]*number[\s\S]*4294967296[\s\S]*four-byte AS number must be[\s\S]*65536-4294967295`),
			},
		},
	})
}

func TestNormalizeBgpRdFunction_MaxTwoByteAssigned(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsNormalizeBgpRd_maxTwoByteAssigned(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("format", "two_byte_as"),
					resource.TestCheckOutput("as_number", "1"),
					resource.TestCheckOutput("assigned_number", "4294967295"),
				),
			},
		},
	})
}

func TestNormalizeBgpRdFunction_MaxFourByteAS(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsNormalizeBgpRd_maxFourByteAS(),
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

func testAccFunctionUtilsNormalizeBgpRd_auto() string {
	return `
locals {
  result = provider::utils::normalize_bgp_rd("auto")
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

func testAccFunctionUtilsNormalizeBgpRd_autoUpperCase() string {
	return `
locals {
  result = provider::utils::normalize_bgp_rd("AUTO")
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

func testAccFunctionUtilsNormalizeBgpRd_autoMixedCase() string {
	return `
locals {
  result = provider::utils::normalize_bgp_rd("Auto")
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

func testAccFunctionUtilsNormalizeBgpRd_twoByteAS() string {
	return `
locals {
  result = provider::utils::normalize_bgp_rd("65000:1001")
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

func testAccFunctionUtilsNormalizeBgpRd_fourByteAS() string {
	return `
locals {
  result = provider::utils::normalize_bgp_rd("4200000001:1003")
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

func testAccFunctionUtilsNormalizeBgpRd_ipv4Address() string {
	return `
locals {
  result = provider::utils::normalize_bgp_rd("192.168.100.1:1002")
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

func testAccFunctionUtilsNormalizeBgpRd_boundaryTwoByteAS() string {
	return `
locals {
  result = provider::utils::normalize_bgp_rd("65535:100")
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

func testAccFunctionUtilsNormalizeBgpRd_boundaryFourByteAS() string {
	return `
locals {
  result = provider::utils::normalize_bgp_rd("65536:100")
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

func testAccFunctionUtilsNormalizeBgpRd_minTwoByteAS() string {
	return `
locals {
  result = provider::utils::normalize_bgp_rd("1:0")
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

func testAccFunctionUtilsNormalizeBgpRd_invalidNoColon() string {
	return `
output "invalid" {
  value = provider::utils::normalize_bgp_rd("65000")
}
`
}

func testAccFunctionUtilsNormalizeBgpRd_invalidLeftSide() string {
	return `
output "invalid" {
  value = provider::utils::normalize_bgp_rd("abc:100")
}
`
}

func testAccFunctionUtilsNormalizeBgpRd_invalidRightSide() string {
	return `
output "invalid" {
  value = provider::utils::normalize_bgp_rd("65000:abc")
}
`
}

func testAccFunctionUtilsNormalizeBgpRd_invalidIPv4() string {
	return `
output "invalid" {
  value = provider::utils::normalize_bgp_rd("999.999.999.999:100")
}
`
}

func testAccFunctionUtilsNormalizeBgpRd_invalidZeroAS() string {
	return `
output "invalid" {
  value = provider::utils::normalize_bgp_rd("0:100")
}
`
}

func testAccFunctionUtilsNormalizeBgpRd_invalidNegativeAS() string {
	return `
output "invalid" {
  value = provider::utils::normalize_bgp_rd("-1:100")
}
`
}

func testAccFunctionUtilsNormalizeBgpRd_invalidNegativeAssigned() string {
	return `
output "invalid" {
  value = provider::utils::normalize_bgp_rd("65000:-1")
}
`
}

func testAccFunctionUtilsNormalizeBgpRd_invalidFourByteAssignedTooLarge() string {
	return `
output "invalid" {
  value = provider::utils::normalize_bgp_rd("100000:70000")
}
`
}

func testAccFunctionUtilsNormalizeBgpRd_invalidIPv4AssignedTooLarge() string {
	return `
output "invalid" {
  value = provider::utils::normalize_bgp_rd("10.0.0.1:70000")
}
`
}

func testAccFunctionUtilsNormalizeBgpRd_invalidASTooLarge() string {
	return `
output "invalid" {
  value = provider::utils::normalize_bgp_rd("4294967296:100")
}
`
}

func testAccFunctionUtilsNormalizeBgpRd_maxTwoByteAssigned() string {
	return `
locals {
  result = provider::utils::normalize_bgp_rd("1:4294967295")
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

func testAccFunctionUtilsNormalizeBgpRd_maxFourByteAS() string {
	return `
locals {
  result = provider::utils::normalize_bgp_rd("4294967295:65535")
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
