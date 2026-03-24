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
  yaml_input = <<-EOT
    name: example
    database: !env DATABASE_URL
    settings:
      debug: true
      timeout: 30
      tags:
        - web
        - production
  EOT
}

# Decode YAML string to Terraform value
# Unknown tags like !env are preserved as literal strings
output "decoded" {
  value = provider::utils::yaml_decode(local.yaml_input)
}

/*
decoded = {
  "database" = "!env DATABASE_URL"
  "name"     = "example"
  "settings" = {
    "debug"   = true
    "tags"    = [
      "web",
      "production",
    ]
    "timeout" = 30
  }
}
*/
