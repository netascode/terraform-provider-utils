output "mask_24" {
  value = provider::utils::normalize_mask(24, "dotted-decimal")
  # Output: "255.255.255.0"
}

output "mask_0" {
  value = provider::utils::normalize_mask(0, "dotted-decimal")
  # Output: "0.0.0.0"
}
