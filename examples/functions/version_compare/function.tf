# Basic version comparison
output "compare_equal" {
  value = provider::utils::version_compare("1.2.3", "1.2.3")
  # Output: 0
}

output "compare_greater" {
  value = provider::utils::version_compare("2.0.0", "1.5.0")
  # Output: 1
}

output "compare_less" {
  value = provider::utils::version_compare("1.5.0", "2.0.0")
  # Output: -1
}

# Real-world example: conditional resource configuration based on version
locals {
  current_version  = "24.4.1"
  required_version = "25.2.2"
}

output "feature_available" {
  value = provider::utils::version_compare(local.current_version, local.required_version) >= 0
  # Output: false (24.4.1 < 25.2.2)
}

output "comparison_result" {
  value = provider::utils::version_compare(local.current_version, local.required_version)
  # Output: -1
}

# Version comparison with 'v' prefix support
output "compare_with_v_prefix" {
  value = provider::utils::version_compare("v1.2.3", "v1.2.4")
  # Output: -1
}

# Practical use case: determine if a feature should be enabled
output "should_use_new_feature" {
  value = provider::utils::version_compare("25.3.0", "25.2.2") > 0 ? "enabled" : "disabled"
  # Output: "enabled"
}

