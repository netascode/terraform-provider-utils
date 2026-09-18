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
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

func TestRenderDeviceConfigsFunction_EmptyYamlInputs(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRenderDeviceConfigs_emptyYamlInputs(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("device_name", "spine1"),
					resource.TestCheckOutput("hostname", "spine1"),
				),
			},
		},
	})
}

func testAccRenderDeviceConfigs_emptyYamlInputs() string {
	return `
	locals {
		model = {
			nxos = {
				devices = [
					{
						name = "spine1"
						configuration = {
							system = {
								hostname = "spine1"
							}
						}
					}
				]
			}
		}

		result = provider::utils::render_device_configs(
			["", "# just a comment\n", "   \n  \n"],
			local.model, "", {}, [], []
		)
		device = local.result.raw.nxos.devices[0]
	}

	output "device_name" {
		value = local.device.name
	}
	output "hostname" {
		value = local.device.configuration.system.hostname
	}
	`
}

func TestRenderDeviceConfigsFunction_Basic(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRenderDeviceConfigs_basic(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("device_name", "spine1"),
					resource.TestCheckOutput("device_managed", "true"),
					resource.TestCheckOutput("device_url", "https://spine1.example.com"),
					resource.TestCheckOutput("hostname", "spine1"),
				),
			},
		},
	})
}

func testAccRenderDeviceConfigs_basic() string {
	return `
	locals {
		model = {
			nxos = {
				devices = [
					{
						name = "spine1"
						url  = "https://spine1.example.com"
						configuration = {
							system = {
								hostname = "spine1"
							}
						}
					}
				]
			}
		}

		result = provider::utils::render_device_configs([], local.model, "", {}, [], [])
		device = local.result.raw.nxos.devices[0]
	}

	output "device_name" {
		value = local.device.name
	}
	output "device_managed" {
		value = tostring(local.device.managed)
	}
	output "device_url" {
		value = local.device.url
	}
	output "hostname" {
		value = local.device.configuration.system.hostname
	}
	`
}

func TestRenderDeviceConfigsFunction_Precedence(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRenderDeviceConfigs_precedence(),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Device config wins over group and global
					resource.TestCheckOutput("priority_val", "device"),
					// Group config wins over global
					resource.TestCheckOutput("group_val", "group"),
					// Global value preserved when not overridden
					resource.TestCheckOutput("global_only", "global_data"),
				),
			},
		},
	})
}

func testAccRenderDeviceConfigs_precedence() string {
	return `
	locals {
		model = {
			nxos = {
				global = {
					configuration = {
						system = {
							priority  = "global"
							group_key = "global"
							global_only = "global_data"
						}
					}
				}
				device_groups = [
					{
						name = "spines"
						devices = ["spine1"]
						configuration = {
							system = {
								priority  = "group"
								group_key = "group"
							}
						}
					}
				]
				devices = [
					{
						name = "spine1"
						configuration = {
							system = {
								priority = "device"
							}
						}
					}
				]
			}
		}

		result = provider::utils::render_device_configs([], local.model, "", {}, [], [])
		device = local.result.raw.nxos.devices[0]
	}

	output "priority_val" {
		value = local.device.configuration.system.priority
	}
	output "group_val" {
		value = local.device.configuration.system.group_key
	}
	output "global_only" {
		value = local.device.configuration.system.global_only
	}
	`
}

func TestRenderDeviceConfigsFunction_ModelTemplates(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRenderDeviceConfigs_modelTemplates(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("rendered_hostname", "spine1.example.com"),
				),
			},
		},
	})
}

func testAccRenderDeviceConfigs_modelTemplates() string {
	return `
	locals {
		model = {
			nxos = {
				templates = [
					{
						name = "base"
						type = "model"
						configuration = {
							system = {
								hostname = "$${name}.$${domain}"
							}
						}
					}
				]
				global = {
					templates  = ["base"]
					variables = {
						domain = "example.com"
					}
				}
				devices = [
					{
						name = "spine1"
						variables = {
							name = "spine1"
						}
						configuration = {}
					}
				]
			}
		}

		result = provider::utils::render_device_configs([], local.model, "", {}, [], [])
		device = local.result.raw.nxos.devices[0]
	}

	output "rendered_hostname" {
		value = local.device.configuration.system.hostname
	}
	`
}

func TestRenderDeviceConfigsFunction_DeviceFilter(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRenderDeviceConfigs_deviceFilter(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("device_count", "1"),
					resource.TestCheckOutput("device_name", "spine1"),
				),
			},
		},
	})
}

func testAccRenderDeviceConfigs_deviceFilter() string {
	return `
	locals {
		model = {
			nxos = {
				devices = [
					{
						name = "spine1"
						configuration = {}
					},
					{
						name = "spine2"
						configuration = {}
					},
					{
						name = "leaf1"
						configuration = {}
					}
				]
			}
		}

		result = provider::utils::render_device_configs([], local.model, "", {}, ["spine1"], [])
		devices = local.result.raw.nxos.devices
	}

	output "device_count" {
		value = tostring(length(local.devices))
	}
	output "device_name" {
		value = local.devices[0].name
	}
	`
}

func TestRenderDeviceConfigsFunction_GroupFilter(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRenderDeviceConfigs_groupFilter(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("device_count", "2"),
				),
			},
		},
	})
}

func testAccRenderDeviceConfigs_groupFilter() string {
	return `
	locals {
		model = {
			nxos = {
				device_groups = [
					{
						name    = "spines"
						devices = ["spine1", "spine2"]
					}
				]
				devices = [
					{
						name = "spine1"
						configuration = {}
					},
					{
						name = "spine2"
						configuration = {}
					},
					{
						name = "leaf1"
						configuration = {}
					}
				]
			}
		}

		result = provider::utils::render_device_configs([], local.model, "", {}, [], ["spines"])
		devices = local.result.raw.nxos.devices
	}

	output "device_count" {
		value = tostring(length(local.devices))
	}
	`
}

func TestRenderDeviceConfigsFunction_InterfaceGroups(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRenderDeviceConfigs_interfaceGroups(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("eth_mtu", "9216"),
					resource.TestCheckOutput("eth_name", "Ethernet1/1"),
				),
			},
		},
	})
}

func testAccRenderDeviceConfigs_interfaceGroups() string {
	return `
	locals {
		model = {
			nxos = {
				interface_groups = [
					{
						name = "fabric"
						configuration = {
							mtu = 9216
						}
					}
				]
				devices = [
					{
						name = "spine1"
						configuration = {
							interfaces = {
								ethernets = [
									{
										name = "Ethernet1/1"
										interface_groups = ["fabric"]
									}
								]
							}
						}
					}
				]
			}
		}

		result = provider::utils::render_device_configs([], local.model, "", {}, [], [])
		device = local.result.raw.nxos.devices[0]
		eth = local.device.configuration.interfaces.ethernets[0]
	}

	output "eth_mtu" {
		value = tostring(local.eth.mtu)
	}
	output "eth_name" {
		value = local.eth.name
	}
	`
}

func TestRenderDeviceConfigsFunction_InterfaceGroupsNestedMapIsolation(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRenderDeviceConfigs_interfaceGroupsNestedMapIsolation(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("eth0_ip", "172.25.6.233/31"),
					resource.TestCheckOutput("eth1_ip", "172.25.6.231/31"),
				),
			},
		},
	})
}

func testAccRenderDeviceConfigs_interfaceGroupsNestedMapIsolation() string {
	return `
	locals {
		model = {
			nxos = {
				interface_groups = [
					{
						name = "fabric"
						configuration = {
							mtu = 9216
							ip = {
								redirects = false
							}
						}
					}
				]
				devices = [
					{
						name = "spine1"
						configuration = {
							interfaces = {
								ethernets = [
									{
										id               = "1/33"
										interface_groups = ["fabric"]
										ip = {
											address = "172.25.6.233/31"
										}
									},
									{
										id               = "1/34"
										interface_groups = ["fabric"]
										ip = {
											address = "172.25.6.231/31"
										}
									}
								]
							}
						}
					}
				]
			}
		}

		result = provider::utils::render_device_configs([], local.model, "", {}, [], [])
		device  = local.result.raw.nxos.devices[0]
		eths    = local.device.configuration.interfaces.ethernets
	}

	output "eth0_ip" {
		value = local.eths[0].ip.address
	}
	output "eth1_ip" {
		value = local.eths[1].ip.address
	}
	`
}

func TestRenderDeviceConfigsFunction_GlobalConfigIsolation(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRenderDeviceConfigs_globalConfigIsolation(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("spine1_ntp_server", "172.0.0.1"),
					resource.TestCheckOutput("spine2_ntp_server", "10.0.0.1"),
				),
			},
		},
	})
}

func testAccRenderDeviceConfigs_globalConfigIsolation() string {
	return `
	locals {
		model = {
			nxos = {
				global = {
					configuration = {
						ntp = {
							server = "10.0.0.1"
						}
					}
				}
				devices = [
					{
						name = "spine1"
						configuration = {
							ntp = {
								server = "172.0.0.1"
							}
						}
					},
					{
						name = "spine2"
						configuration = {}
					}
				]
			}
		}

		result  = provider::utils::render_device_configs([], local.model, "", {}, [], [])
		devices = local.result.raw.nxos.devices
	}

	output "spine1_ntp_server" {
		value = local.devices[0].configuration.ntp.server
	}
	output "spine2_ntp_server" {
		value = local.devices[1].configuration.ntp.server
	}
	`
}

func TestRenderDeviceConfigsFunction_GroupConfigIsolation(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRenderDeviceConfigs_groupConfigIsolation(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("spine1_ntp_server", "172.0.0.1"),
					resource.TestCheckOutput("spine2_ntp_server", "10.0.0.1"),
				),
			},
		},
	})
}

func testAccRenderDeviceConfigs_groupConfigIsolation() string {
	return `
	locals {
		model = {
			nxos = {
				device_groups = [
					{
						name    = "spines"
						devices = ["spine1", "spine2"]
						configuration = {
							ntp = {
								server = "10.0.0.1"
							}
						}
					}
				]
				devices = [
					{
						name = "spine1"
						configuration = {
							ntp = {
								server = "172.0.0.1"
							}
						}
					},
					{
						name = "spine2"
						configuration = {}
					}
				]
			}
		}

		result  = provider::utils::render_device_configs([], local.model, "", {}, [], [])
		devices = local.result.raw.nxos.devices
	}

	output "spine1_ntp_server" {
		value = local.devices[0].configuration.ntp.server
	}
	output "spine2_ntp_server" {
		value = local.devices[1].configuration.ntp.server
	}
	`
}

func TestRenderDeviceConfigsFunction_CliTemplates(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRenderDeviceConfigs_cliTemplates(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("cli_count", "2"),
					resource.TestCheckOutput("cli_name_0", "base_cli"),
					resource.TestCheckOutput("cli_content_0", "hostname spine1"),
				),
			},
		},
	})
}

func testAccRenderDeviceConfigs_cliTemplates() string {
	return `
	locals {
		model = {
			nxos = {
				templates = [
					{
						name    = "base_cli"
						type    = "cli"
						content = "hostname $${name}"
						order   = 10
					}
				]
				global = {
					templates = ["base_cli"]
				}
				devices = [
					{
						name = "spine1"
						variables = {
							name = "spine1"
						}
						configuration = {}
						cli_templates = [
							{
								name    = "extra"
								content = "logging server 10.0.0.1"
								order   = 20
							}
						]
					}
				]
			}
		}

		result = provider::utils::render_device_configs([], local.model, "", {}, [], [])
		device = local.result.raw.nxos.devices[0]
	}

	output "cli_count" {
		value = tostring(length(local.device.cli_templates))
	}
	output "cli_name_0" {
		value = local.device.cli_templates[0].name
	}
	output "cli_content_0" {
		value = local.device.cli_templates[0].content
	}
	`
}

func TestRenderDeviceConfigsFunction_IosxeArch(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRenderDeviceConfigs_iosxe(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("device_name", "router1"),
				),
			},
		},
	})
}

func testAccRenderDeviceConfigs_iosxe() string {
	return `
	locals {
		model = {
			iosxe = {
				devices = [
					{
						name = "router1"
						configuration = {
							system = {
								hostname = "router1"
							}
						}
					}
				]
			}
		}

		result = provider::utils::render_device_configs([], local.model, "", {}, [], [])
		device = local.result.raw.iosxe.devices[0]
	}

	output "device_name" {
		value = local.device.name
	}
	`
}

func TestRenderDeviceConfigsFunction_Subinterfaces(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRenderDeviceConfigs_subinterfaces(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("sub_mtu", "9216"),
					resource.TestCheckOutput("sub_vlan", "100"),
				),
			},
		},
	})
}

func testAccRenderDeviceConfigs_subinterfaces() string {
	return `
	locals {
		model = {
			nxos = {
				interface_groups = [
					{
						name = "fabric_sub"
						configuration = {
							mtu = 9216
						}
					}
				]
				devices = [
					{
						name = "spine1"
						configuration = {
							interfaces = {
								ethernets = [
									{
										name = "Ethernet1/1"
										subinterfaces = [
											{
												vlan = 100
												interface_groups = ["fabric_sub"]
											}
										]
									}
								]
							}
						}
					}
				]
			}
		}

		result = provider::utils::render_device_configs([], local.model, "", {}, [], [])
		device = local.result.raw.nxos.devices[0]
		sub = local.device.configuration.interfaces.ethernets[0].subinterfaces[0]
	}

	output "sub_mtu" {
		value = tostring(local.sub.mtu)
	}
	output "sub_vlan" {
		value = tostring(local.sub.vlan)
	}
	`
}

func TestRenderDeviceConfigsFunction_GlobalVariable(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRenderDeviceConfigs_globalVariable(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("first_device_ref", "spine1"),
				),
			},
		},
	})
}

func testAccRenderDeviceConfigs_globalVariable() string {
	return `
	locals {
		model = {
			nxos = {
				templates = [
					{
						name = "ref_tmpl"
						type = "model"
						configuration = {
							system = {
								first_device = "$${GLOBAL.devices[0].name}"
							}
						}
					}
				]
				global = {
					templates = ["ref_tmpl"]
				}
				devices = [
					{
						name = "spine1"
						configuration = {}
					}
				]
			}
		}

		result = provider::utils::render_device_configs([], local.model, "", {}, [], [])
		device = local.result.raw.nxos.devices[0]
	}

	output "first_device_ref" {
		value = local.device.configuration.system.first_device
	}
	`
}

func TestRenderDeviceConfigsFunction_EmptySections(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRenderDeviceConfigs_emptySections(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("device_name", "spine1"),
					resource.TestCheckOutput("cli_count", "0"),
				),
			},
		},
	})
}

func testAccRenderDeviceConfigs_emptySections() string {
	return `
	locals {
		model = {
			nxos = {
				devices = [
					{
						name = "spine1"
						configuration = {}
					}
				]
			}
		}

		result = provider::utils::render_device_configs([], local.model, "", {}, [], [])
		device = local.result.raw.nxos.devices[0]
	}

	output "device_name" {
		value = local.device.name
	}
	output "cli_count" {
		value = tostring(length(local.device.cli_templates))
	}
	`
}

func TestRenderDeviceConfigsFunction_BidirectionalGroups(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRenderDeviceConfigs_bidirectionalGroups(),
				Check: resource.ComposeAggregateTestCheckFunc(
					// spine1 is in group via group.devices list
					resource.TestCheckOutput("spine1_group_val", "spines_data"),
					// spine2 is in group via device.device_groups list
					resource.TestCheckOutput("spine2_group_val", "spines_data"),
				),
			},
		},
	})
}

func testAccRenderDeviceConfigs_bidirectionalGroups() string {
	return `
	locals {
		model = {
			nxos = {
				device_groups = [
					{
						name    = "spines"
						devices = ["spine1"]
						configuration = {
							system = {
								group_val = "spines_data"
							}
						}
					}
				]
				devices = [
					{
						name = "spine1"
						configuration = {}
					},
					{
						name = "spine2"
						device_groups = ["spines"]
						configuration = {}
					}
				]
			}
		}

		result = provider::utils::render_device_configs([], local.model, "", {}, [], [])
		devices = local.result.raw.nxos.devices
	}

	output "spine1_group_val" {
		value = local.devices[0].configuration.system.group_val
	}
	output "spine2_group_val" {
		value = local.devices[1].configuration.system.group_val
	}
	`
}

func TestRenderDeviceConfigsFunction_Defaults(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRenderDeviceConfigs_defaults(),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Device overrides default
					resource.TestCheckOutput("feature_bgp", "true"),
					// Default applied (not in device config)
					resource.TestCheckOutput("feature_ospf", "false"),
					// List-item default applied to VRF
					resource.TestCheckOutput("vrf_description", "default-desc"),
					// VRF name preserved (not overridden by defaults)
					resource.TestCheckOutput("vrf_name", "vrf1"),
					// Default managed from defaults parameter
					resource.TestCheckOutput("device_managed", "true"),
				),
			},
		},
	})
}

func testAccRenderDeviceConfigs_defaults() string {
	return `
	locals {
		model = {
			nxos = {
				devices = [
					{
						name = "spine1"
						url  = "https://spine1.example.com"
						configuration = {
							feature = {
								bgp = true
							}
							vrfs = [
								{
									name = "vrf1"
								}
							]
						}
					}
				]
			}
		}

		defaults_yaml = <<-EOT
defaults:
  nxos:
    devices:
      managed: true
      configuration:
        feature:
          bgp: false
          ospf: false
        vrfs:
          description: default-desc
    templates:
      order: 0
EOT

		result = provider::utils::render_device_configs([], local.model, local.defaults_yaml, {}, [], [])
		device = local.result.raw.nxos.devices[0]
	}

	output "feature_bgp" {
		value = tostring(local.device.configuration.feature.bgp)
	}
	output "feature_ospf" {
		value = tostring(local.device.configuration.feature.ospf)
	}
	output "vrf_description" {
		value = local.device.configuration.vrfs[0].description
	}
	output "vrf_name" {
		value = local.device.configuration.vrfs[0].name
	}
	output "device_managed" {
		value = tostring(local.device.managed)
	}
	`
}

func TestRenderDeviceConfigsFunction_YamlMerge(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRenderDeviceConfigs_yamlMerge(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("hostname", "spine1"),
					resource.TestCheckOutput("dns_server", "10.0.0.53"),
				),
			},
		},
	})
}

func testAccRenderDeviceConfigs_yamlMerge() string {
	return `
	locals {
		yaml1 = <<-EOT
nxos:
  global:
    configuration:
      system:
        dns_server: "10.0.0.53"
  devices:
    - name: spine1
      configuration:
        system:
          hostname: spine1
EOT

		result = provider::utils::render_device_configs([local.yaml1], {}, "", {}, [], [])
		device = local.result.raw.nxos.devices[0]
	}

	output "hostname" {
		value = local.device.configuration.system.hostname
	}
	output "dns_server" {
		value = local.device.configuration.system.dns_server
	}
	`
}

func TestRenderDeviceConfigsFunction_ProviderDevices(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRenderDeviceConfigs_providerDevices(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("pd_count", "2"),
					resource.TestCheckOutput("pd0_name", "spine1"),
					resource.TestCheckOutput("pd0_url", "https://spine1.example.com"),
					resource.TestCheckOutput("pd0_managed", "true"),
					resource.TestCheckOutput("pd1_name", "spine2"),
					resource.TestCheckOutput("pd1_managed", "true"),
				),
			},
		},
	})
}

func testAccRenderDeviceConfigs_providerDevices() string {
	return `
	locals {
		model = {
			nxos = {
				devices = [
					{
						name = "spine1"
						url  = "https://spine1.example.com"
						configuration = {}
					},
					{
						name = "spine2"
						configuration = {}
					}
				]
			}
		}

		result = provider::utils::render_device_configs([], local.model, "", {}, [], [])
		pd     = local.result.provider_devices
	}

	output "pd_count" {
		value = tostring(length(local.pd))
	}
	output "pd0_name" {
		value = local.pd[0].name
	}
	output "pd0_url" {
		value = local.pd[0].url
	}
	output "pd0_managed" {
		value = tostring(local.pd[0].managed)
	}
	output "pd1_name" {
		value = local.pd[1].name
	}
	output "pd1_managed" {
		value = tostring(local.pd[1].managed)
	}
	`
}

func TestRenderDeviceConfigsFunction_NullStripping(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRenderDeviceConfigs_nullStripping(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("device_name", "spine1"),
					resource.TestCheckOutput("has_url", "false"),
				),
			},
		},
	})
}

func testAccRenderDeviceConfigs_nullStripping() string {
	return `
	locals {
		model = {
			nxos = {
				devices = [
					{
						name = "spine1"
						configuration = {}
					}
				]
			}
		}

		result = provider::utils::render_device_configs([], local.model, "", {}, [], [])
		device = local.result.raw.nxos.devices[0]
	}

	output "device_name" {
		value = local.device.name
	}
	output "has_url" {
		value = tostring(can(local.device.url))
	}
	`
}

func TestRenderDeviceConfigsFunction_DefaultsMerge(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRenderDeviceConfigs_defaultsMerge(),
				Check: resource.ComposeAggregateTestCheckFunc(
					// User default overrides module default
					resource.TestCheckOutput("feature_ospf", "true"),
					// Module default applied (not overridden by user)
					resource.TestCheckOutput("feature_bgp", "false"),
				),
			},
		},
	})
}

func testAccRenderDeviceConfigs_defaultsMerge() string {
	return `
	locals {
		model = {
			defaults = {
				nxos = {
					devices = {
						configuration = {
							feature = {
								ospf = true
							}
						}
					}
				}
			}
			nxos = {
				devices = [
					{
						name = "spine1"
						configuration = {}
					}
				]
			}
		}

		defaults_yaml = <<-EOT
defaults:
  nxos:
    devices:
      managed: true
      configuration:
        feature:
          bgp: false
          ospf: false
    templates:
      order: 0
EOT

		result = provider::utils::render_device_configs([], local.model, local.defaults_yaml, {}, [], [])
		device = local.result.raw.nxos.devices[0]
	}

	output "feature_ospf" {
		value = tostring(local.device.configuration.feature.ospf)
	}
	output "feature_bgp" {
		value = tostring(local.device.configuration.feature.bgp)
	}
	`
}

func TestRenderDeviceConfigsFunction_InterfaceGroupPolicyReplace(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRenderDeviceConfigs_interfaceGroupPolicyReplace(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("eth1_mtu", "9216"),
					resource.TestCheckOutput("eth1_description", "TRUNK_VIA_GROUP"),
					resource.TestCheckOutput("eth2_mtu", "1500"),
					resource.TestCheckOutput("eth2_description", "ACCESS_VIA_GROUP"),
				),
			},
		},
	})
}

func testAccRenderDeviceConfigs_interfaceGroupPolicyReplace() string {
	return `
	locals {
		model = {
			iosxe = {
				interface_groups = [
					{
						name = "access"
						configuration = {
							mtu = 1500
							description = "ACCESS_VIA_GROUP"
						}
					},
					{
						name = "trunk"
						configuration = {
							mtu = 9216
							description = "TRUNK_VIA_GROUP"
						}
					}
				]
				device_groups = [
					{
						name    = "switches"
						devices = ["switch1"]
						configuration = {
							interfaces = {
								ethernets = [
									{
										type = "GigabitEthernet"
										id   = "1/0/1"
										interface_groups = ["access"]
									},
									{
										type = "GigabitEthernet"
										id   = "1/0/2"
										interface_groups = ["access"]
									}
								]
							}
						}
					}
				]
				devices = [
					{
						name = "switch1"
						interface_group_policy = "replace"
						configuration = {
							interfaces = {
								ethernets = [
									{
										type = "GigabitEthernet"
										id   = "1/0/1"
										interface_groups = ["trunk"]
									}
								]
							}
						}
					}
				]
			}
		}

		result = provider::utils::render_device_configs([], local.model, "", {}, [], [])
		device = local.result.raw.iosxe.devices[0]
		eth1   = [for e in local.device.configuration.interfaces.ethernets : e if e.type == "GigabitEthernet" && e.id == "1/0/1"][0]
		eth2   = [for e in local.device.configuration.interfaces.ethernets : e if e.type == "GigabitEthernet" && e.id == "1/0/2"][0]
	}

	output "eth1_mtu" {
		value = tostring(local.eth1.mtu)
	}
	output "eth1_description" {
		value = local.eth1.description
	}
	output "eth2_mtu" {
		value = tostring(local.eth2.mtu)
	}
	output "eth2_description" {
		value = local.eth2.description
	}
	`
}

func TestRenderDeviceConfigsFunction_InterfaceGroupPolicyMergeDefault(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRenderDeviceConfigs_interfaceGroupPolicyMergeDefault(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("eth1_mtu", "9216"),
					resource.TestCheckOutput("eth1_shutdown", "true"),
				),
			},
		},
	})
}

func testAccRenderDeviceConfigs_interfaceGroupPolicyMergeDefault() string {
	return `
	locals {
		model = {
			iosxe = {
				interface_groups = [
					{
						name = "base"
						configuration = {
							shutdown = true
						}
					},
					{
						name = "fabric"
						configuration = {
							mtu = 9216
						}
					}
				]
				device_groups = [
					{
						name    = "switches"
						devices = ["switch1"]
						configuration = {
							interfaces = {
								ethernets = [
									{
										type = "GigabitEthernet"
										id   = "1/0/1"
										interface_groups = ["base"]
									}
								]
							}
						}
					}
				]
				devices = [
					{
						name = "switch1"
						configuration = {
							interfaces = {
								ethernets = [
									{
										type = "GigabitEthernet"
										id   = "1/0/1"
										interface_groups = ["fabric"]
									}
								]
							}
						}
					}
				]
			}
		}

		result = provider::utils::render_device_configs([], local.model, "", {}, [], [])
		device = local.result.raw.iosxe.devices[0]
		eth1   = local.device.configuration.interfaces.ethernets[0]
	}

	output "eth1_mtu" {
		value = tostring(local.eth1.mtu)
	}
	output "eth1_shutdown" {
		value = tostring(local.eth1.shutdown)
	}
	`
}

func TestRenderDeviceConfigsFunction_InterfaceGroupPolicyPerInterface(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRenderDeviceConfigs_interfaceGroupPolicyPerInterface(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("eth1_mtu", "9216"),
					resource.TestCheckOutput("eth2_mtu", "1500"),
					resource.TestCheckOutput("eth2_description", "FROM_ACCESS_GROUP"),
				),
			},
		},
	})
}

func testAccRenderDeviceConfigs_interfaceGroupPolicyPerInterface() string {
	return `
	locals {
		model = {
			iosxe = {
				interface_groups = [
					{
						name = "access"
						configuration = {
							mtu = 1500
							shutdown = true
							description = "FROM_ACCESS_GROUP"
						}
					},
					{
						name = "trunk"
						configuration = {
							mtu = 9216
						}
					},
					{
						name = "monitor"
						configuration = {
							mtu = 1500
							shutdown = false
						}
					}
				]
				device_groups = [
					{
						name    = "switches"
						devices = ["switch1"]
						configuration = {
							interfaces = {
								ethernets = [
									{
										type = "GigabitEthernet"
										id   = "1/0/1"
										interface_groups = ["access"]
									},
									{
										type = "GigabitEthernet"
										id   = "1/0/2"
										interface_groups = ["access"]
									}
								]
							}
						}
					}
				]
				devices = [
					{
						name = "switch1"
						interface_group_policy = "replace"
						configuration = {
							interfaces = {
								ethernets = [
									{
										type = "GigabitEthernet"
										id   = "1/0/1"
										interface_groups = ["trunk"]
									},
									{
										type = "GigabitEthernet"
										id   = "1/0/2"
										interface_group_policy = "merge"
										interface_groups = ["monitor"]
									}
								]
							}
						}
					}
				]
			}
		}

		result = provider::utils::render_device_configs([], local.model, "", {}, [], [])
		device = local.result.raw.iosxe.devices[0]
		eth1   = [for e in local.device.configuration.interfaces.ethernets : e if e.type == "GigabitEthernet" && e.id == "1/0/1"][0]
		eth2   = [for e in local.device.configuration.interfaces.ethernets : e if e.type == "GigabitEthernet" && e.id == "1/0/2"][0]
	}

	output "eth1_mtu" {
		value = tostring(local.eth1.mtu)
	}
	output "eth2_mtu" {
		value = tostring(local.eth2.mtu)
	}
	output "eth2_description" {
		value = local.eth2.description
	}
	`
}

func TestRenderDeviceConfigsFunction_InterfaceGroupPolicyInheritOnly(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRenderDeviceConfigs_interfaceGroupPolicyInheritOnly(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("eth1_mtu", "9216"),
					resource.TestCheckOutput("eth2_description", "ACCESS_PORT"),
				),
			},
		},
	})
}

func testAccRenderDeviceConfigs_interfaceGroupPolicyInheritOnly() string {
	return `
	locals {
		model = {
			iosxe = {
				interface_groups = [
					{
						name = "access"
						configuration = {
							description = "ACCESS_PORT"
						}
					},
					{
						name = "trunk"
						configuration = {
							mtu = 9216
						}
					}
				]
				device_groups = [
					{
						name    = "switches"
						devices = ["switch1"]
						configuration = {
							interfaces = {
								ethernets = [
									{
										type = "GigabitEthernet"
										id   = "1/0/1"
										interface_groups = ["access"]
									},
									{
										type = "GigabitEthernet"
										id   = "1/0/2"
										interface_groups = ["access"]
									}
								]
							}
						}
					}
				]
				devices = [
					{
						name = "switch1"
						interface_group_policy = "replace"
						configuration = {
							interfaces = {
								ethernets = [
									{
										type = "GigabitEthernet"
										id   = "1/0/1"
										interface_groups = ["trunk"]
									}
								]
							}
						}
					}
				]
			}
		}

		result = provider::utils::render_device_configs([], local.model, "", {}, [], [])
		device = local.result.raw.iosxe.devices[0]
		eth1   = [for e in local.device.configuration.interfaces.ethernets : e if e.type == "GigabitEthernet" && e.id == "1/0/1"][0]
		eth2   = [for e in local.device.configuration.interfaces.ethernets : e if e.type == "GigabitEthernet" && e.id == "1/0/2"][0]
	}

	output "eth1_mtu" {
		value = tostring(local.eth1.mtu)
	}
	output "eth2_description" {
		value = local.eth2.description
	}
	`
}

func TestRenderDeviceConfigsFunction_InterfaceGroupPolicyEmptyList(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRenderDeviceConfigs_interfaceGroupPolicyEmptyList(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("eth1_has_mtu", "false"),
					resource.TestCheckOutput("eth1_description", "STRIPPED"),
				),
			},
		},
	})
}

func testAccRenderDeviceConfigs_interfaceGroupPolicyEmptyList() string {
	return `
	locals {
		model = {
			iosxe = {
				interface_groups = [
					{
						name = "access"
						configuration = {
							mtu = 1500
							description = "ACCESS_PORT"
						}
					}
				]
				device_groups = [
					{
						name    = "switches"
						devices = ["switch1"]
						configuration = {
							interfaces = {
								ethernets = [
									{
										type = "GigabitEthernet"
										id   = "1/0/1"
										interface_groups = ["access"]
									}
								]
							}
						}
					}
				]
				devices = [
					{
						name = "switch1"
						interface_group_policy = "replace"
						configuration = {
							interfaces = {
								ethernets = [
									{
										type = "GigabitEthernet"
										id   = "1/0/1"
										interface_groups = []
										description = "STRIPPED"
									}
								]
							}
						}
					}
				]
			}
		}

		result = provider::utils::render_device_configs([], local.model, "", {}, [], [])
		device = local.result.raw.iosxe.devices[0]
		eth1   = local.device.configuration.interfaces.ethernets[0]
	}

	output "eth1_has_mtu" {
		value = tostring(can(local.eth1.mtu))
	}
	output "eth1_description" {
		value = local.eth1.description
	}
	`
}

func TestRenderDeviceConfigsFunction_InterfaceGroupPolicySubinterface(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRenderDeviceConfigs_interfaceGroupPolicySubinterface(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("sub10_mtu", "9216"),
					resource.TestCheckOutput("sub20_description", "BASE_VIA_GROUP"),
				),
			},
		},
	})
}

func testAccRenderDeviceConfigs_interfaceGroupPolicySubinterface() string {
	return `
	locals {
		model = {
			iosxe = {
				interface_groups = [
					{
						name = "base_sub"
						configuration = {
							description = "BASE_VIA_GROUP"
							mtu = 1500
						}
					},
					{
						name = "fabric_sub"
						configuration = {
							mtu = 9216
						}
					}
				]
				device_groups = [
					{
						name    = "routers"
						devices = ["router1"]
						configuration = {
							interfaces = {
								port_channels = [
									{
										id = 1
										subinterfaces = [
											{
												id = "10"
												interface_groups = ["base_sub"]
											},
											{
												id = "20"
												interface_groups = ["base_sub"]
											}
										]
									}
								]
							}
						}
					}
				]
				devices = [
					{
						name = "router1"
						interface_group_policy = "replace"
						configuration = {
							interfaces = {
								port_channels = [
									{
										id = 1
										subinterfaces = [
											{
												id = "10"
												interface_groups = ["fabric_sub"]
											}
										]
									}
								]
							}
						}
					}
				]
			}
		}

		result = provider::utils::render_device_configs([], local.model, "", {}, [], [])
		device = local.result.raw.iosxe.devices[0]
		pc1    = [for p in local.device.configuration.interfaces.port_channels : p if p.id == 1][0]
		sub10  = [for s in local.pc1.subinterfaces : s if s.id == "10"][0]
		sub20  = [for s in local.pc1.subinterfaces : s if s.id == "20"][0]
	}

	output "sub10_mtu" {
		value = tostring(local.sub10.mtu)
	}
	output "sub20_description" {
		value = local.sub20.description
	}
	`
}
