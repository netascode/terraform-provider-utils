terraform {
  required_providers {
    utils = {
      source = "netascode/utils"
    }
  }
}

provider "utils" {}

locals {
  model = {
    nxos = {
      templates = [
        {
          name = "base_config"
          type = "model"
          configuration = {
            system = {
              mtu = "$${mtu}"
            }
          }
        }
      ]
      global = {
        templates = ["base_config"]
        variables = {
          mtu = 9216
        }
        configuration = {
          system = {
            dns_server = "10.0.0.53"
          }
        }
      }
      devices = [
        {
          name = "switch1"
          url  = "https://switch1.example.com"
          configuration = {
            system = {
              hostname = "switch1"
            }
          }
        },
        {
          name = "switch2"
          url  = "https://switch2.example.com"
          configuration = {
            system = {
              hostname = "switch2"
            }
          }
        }
      ]
    }
  }

  result = provider::utils::render_device_configs([], local.model, "", {}, [], [])
}

# Each device has fully rendered configuration and metadata
output "switch1" {
  value = local.result.raw.nxos.devices[0]
}

# Provider devices list for configuring the provider
output "provider_devices" {
  value = local.result.provider_devices
}

/*
switch1 = {
  "cli_templates" = []
  "configuration" = {
    "system" = {
      "dns_server" = "10.0.0.53"
      "hostname"   = "switch1"
      "mtu"        = 9216
    }
  }
  "managed" = true
  "name"    = "switch1"
  "url"     = "https://switch1.example.com"
}
*/
