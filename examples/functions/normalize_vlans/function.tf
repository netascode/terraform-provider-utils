# Mixed VLANs with string format
output "mixed_vlans_string" {
  description = "Mixed individual VLANs and ranges as string"
  value = provider::utils::normalize_vlans({
    ids = [1, 2, 5]
    ranges = [
      { from = 10, to = 20 },
      { from = 40, to = 50 }
    ]
  }, "string")
}

# Mixed VLANs with string-nxos format (ranges for 2+ consecutive VLANs)
output "mixed_vlans_string_nxos" {
  description = "Mixed individual VLANs and ranges as NX-OS style string"
  value = provider::utils::normalize_vlans({
    ids = [1, 2, 5]
    ranges = [
      { from = 10, to = 20 },
      { from = 40, to = 50 }
    ]
  }, "string-nxos")
}

# Individual VLANs with list format
output "individual_vlans_list" {
  description = "Individual VLANs as list of integers"
  value = provider::utils::normalize_vlans({
    ids = [100, 200, 300, 400]
  }, "list")
}

# VLAN ranges with list format
output "ranges_list" {
  description = "VLAN ranges expanded as list of integers"
  value = provider::utils::normalize_vlans({
    ranges = [
      { from = 10, to = 12 },
      { from = 20, to = 22 }
    ]
  }, "list")
}

/*
Expected outputs:

mixed_vlans_string = "1,2,5,10-20,40-50"
mixed_vlans_string_nxos = "1-2,5,10-20,40-50"
individual_vlans_list = [100, 200, 300, 400]
ranges_list = [10, 11, 12, 20, 21, 22]

Note: "string" format uses range notation (e.g., "10-20") only for 3 or more consecutive VLANs.
      Individual or pairs of VLANs are listed separately (e.g., "1,2" not "1-2").
      "string-nxos" format uses range notation for 2 or more consecutive VLANs (e.g., "1-2").
*/
