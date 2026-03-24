terraform {
  required_providers {
    utils = {
      source = "netascode/utils"
    }
  }
}

# Configure the provider
provider "utils" {}

locals {
  data = {
    name = "example"
    settings = {
      debug   = true
      timeout = 30
      tags    = ["web", "production"]
    }
  }
}

output "encoded" {
  value = provider::utils::yaml_encode(local.data)
}

/*
encoded = <<-EOT
  name: example
  settings:
    debug: true
    tags:
      - web
      - production
    timeout: 30
EOT
*/
