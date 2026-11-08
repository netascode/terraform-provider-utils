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
    meraki = {
      templates = {
        networks = [
          {
            name = "branch"
            type = "model"
            configuration = {
              time_zone     = "Europe/Berlin"
              notes         = "Branch $${site_id}"
              product_types = ["appliance", "switch"]
            }
          }
        ]
        devices = [
          {
            name = "switch"
            type = "model"
            configuration = {
              address = "$${address}"
              tags    = ["$${role}"]
            }
          }
        ]
      }
      domains = [
        {
          name = "EMEA"
          organizations = [
            {
              name = "Org1"
              networks = [
                {
                  name      = "Branch1"
                  templates = ["branch"]
                  variables = {
                    site_id = 1
                    address = "Main Street 1"
                  }
                  devices = [
                    {
                      serial    = "Q2XX-XXXX-XXXX"
                      name      = "SW1"
                      templates = ["switch"]
                      variables = {
                        role = "access"
                      }
                    }
                  ]
                }
              ]
            }
          ]
        }
      ]
    }
  }

  result = provider::utils::render_meraki_model([], local.model, "", {})
}

# Fully rendered network including its devices
output "network" {
  value = local.result.resolved.meraki.domains[0].organizations[0].networks[0]
}

/*
network = {
  "devices" = [
    {
      "address"   = "Main Street 1"
      "name"      = "SW1"
      "serial"    = "Q2XX-XXXX-XXXX"
      "tags"      = ["access"]
      "templates" = ["switch"]
      "variables" = {
        "role" = "access"
      }
    },
  ]
  "name"          = "Branch1"
  "notes"         = "Branch 1"
  "product_types" = ["appliance", "switch"]
  "templates"     = ["branch"]
  "time_zone"     = "Europe/Berlin"
  "variables" = {
    "address" = "Main Street 1"
    "site_id" = 1
  }
}
*/
