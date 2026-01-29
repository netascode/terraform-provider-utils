# Convert colon-separated format to Cisco dotted notation
output "mac_to_dotted" {
  value = provider::utils::normalize_mac("00:11:22:33:44:55", "dotted")
  # Output: "0011.2233.4455"
}

# Convert Cisco dotted format to IEEE 802 colon format
output "mac_to_colon" {
  value = provider::utils::normalize_mac("0011.2233.4455", "colon")
  # Output: "00:11:22:33:44:55"
}

# Convert any format to dash-separated
output "mac_to_dash" {
  value = provider::utils::normalize_mac("00-11-22-33-44-55", "dash")
  # Output: "00-11-22-33-44-55"
}

# Use in resource configuration
resource "example_network_device" "switch" {
  system_mac = provider::utils::normalize_mac(var.user_provided_mac, "dotted")
}
