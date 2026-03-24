terraform {
  required_providers {
    utils = {
      source = "netascode/utils"
    }
  }
}

# Configure the provider
provider "utils" {}

/*
export DATABASE_URL=postgres://localhost:5432/mydb
*/

locals {
  yaml_input = <<-EOT
    name: myapp
    database: !env DATABASE_URL
    settings:
      debug: true
      timeout: 30
  EOT

  # First decode the YAML (preserves !env tags as literal strings)
  decoded = provider::utils::yaml_decode(local.yaml_input)

  # Then resolve the tags (replaces "!env DATABASE_URL" with the env var value)
  resolved = provider::utils::resolve_yaml_tags(local.decoded)
}

output "resolved" {
  value = local.resolved
}

/*
resolved = {
  "database" = "postgres://localhost:5432/mydb"
  "name"     = "myapp"
  "settings" = {
    "debug"   = true
    "timeout" = 30
  }
}
*/
