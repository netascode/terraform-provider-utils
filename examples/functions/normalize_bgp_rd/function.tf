# Parse an auto RD (BGP RD auto-assignment)
output "rd_auto" {
  value = provider::utils::normalize_bgp_rd("auto")
  # Output: { format = "auto", as_number = 0, assigned_number = 0, ipv4_address = "" }
}

# Parse a Two-byte AS Route Distinguisher (AS <= 65535)
output "rd_two_byte" {
  value = provider::utils::normalize_bgp_rd("65000:1001")
  # Output: { format = "two_byte_as", as_number = 65000, assigned_number = 1001, ipv4_address = "" }
}

# Parse a Four-byte AS Route Distinguisher (AS > 65535)
output "rd_four_byte" {
  value = provider::utils::normalize_bgp_rd("4200000001:1003")
  # Output: { format = "four_byte_as", as_number = 4200000001, assigned_number = 1003, ipv4_address = "" }
}

# Parse an IPv4 Address Route Distinguisher
output "rd_ipv4" {
  value = provider::utils::normalize_bgp_rd("192.168.100.1:1002")
  # Output: { format = "ipv4_address", as_number = 0, assigned_number = 1002, ipv4_address = "192.168.100.1" }
}

# Use in resource configuration
variable "user_rd" {
  type    = string
  default = "65001:100"
}

locals {
  bgp_rd = try(provider::utils::normalize_bgp_rd(var.user_rd), null)
}

resource "example_bgp_rd" "bgp_rd" {
  bgp_rd_auto                = try(local.bgp_rd.format == "auto" ? true : null, null)
  bgp_rd_two_byte_as_number  = try(local.bgp_rd.format == "two_byte_as" ? local.bgp_rd.as_number : null, null)
  bgp_rd_two_byte_as_index   = try(local.bgp_rd.format == "two_byte_as" ? local.bgp_rd.assigned_number : null, null)
  bgp_rd_four_byte_as_number = try(local.bgp_rd.format == "four_byte_as" ? local.bgp_rd.as_number : null, null)
  bgp_rd_four_byte_as_index  = try(local.bgp_rd.format == "four_byte_as" ? local.bgp_rd.assigned_number : null, null)
  bgp_rd_ipv4_address        = try(local.bgp_rd.format == "ipv4_address" ? local.bgp_rd.ipv4_address : null, null)
  bgp_rd_ipv4_address_index  = try(local.bgp_rd.format == "ipv4_address" ? local.bgp_rd.assigned_number : null, null)
}
