# Parse an auto RT (BGP RT auto-assignment)
output "rt_auto" {
  value = provider::utils::normalize_bgp_rt("auto")
  # Output: { format = "auto", as_number = 0, assigned_number = 0, ipv4_address = "" }
}

# Parse a Two-byte AS Route Target (AS <= 65535)
output "rt_two_byte" {
  value = provider::utils::normalize_bgp_rt("65000:1001")
  # Output: { format = "two_byte_as", as_number = 65000, assigned_number = 1001, ipv4_address = "" }
}

# Parse a Four-byte AS Route Target (AS > 65535)
output "rt_four_byte" {
  value = provider::utils::normalize_bgp_rt("4200000001:1003")
  # Output: { format = "four_byte_as", as_number = 4200000001, assigned_number = 1003, ipv4_address = "" }
}

# Parse an IPv4 Address Route Target
output "rt_ipv4" {
  value = provider::utils::normalize_bgp_rt("192.168.100.1:1002")
  # Output: { format = "ipv4_address", as_number = 0, assigned_number = 1002, ipv4_address = "192.168.100.1" }
}

# Use in resource configuration
variable "user_rt" {
  type    = string
  default = "65001:100"
}

locals {
  bgp_rt = try(provider::utils::normalize_bgp_rt(var.user_rt), null)
}

resource "example_bgp_rt" "bgp_rt" {
  bgp_rt_auto                = try(local.bgp_rt.format == "auto" ? true : null, null)
  bgp_rt_two_byte_as_number  = try(local.bgp_rt.format == "two_byte_as" ? local.bgp_rt.as_number : null, null)
  bgp_rt_two_byte_as_index   = try(local.bgp_rt.format == "two_byte_as" ? local.bgp_rt.assigned_number : null, null)
  bgp_rt_four_byte_as_number = try(local.bgp_rt.format == "four_byte_as" ? local.bgp_rt.as_number : null, null)
  bgp_rt_four_byte_as_index  = try(local.bgp_rt.format == "four_byte_as" ? local.bgp_rt.assigned_number : null, null)
  bgp_rt_ipv4_address        = try(local.bgp_rt.format == "ipv4_address" ? local.bgp_rt.ipv4_address : null, null)
  bgp_rt_ipv4_address_index  = try(local.bgp_rt.format == "ipv4_address" ? local.bgp_rt.assigned_number : null, null)
}
