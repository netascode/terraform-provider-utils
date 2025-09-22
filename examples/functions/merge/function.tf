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
  data_1 = {
    root = {
      elem1 = "value1"
      child1 = {
        cc1 = 1
      }
    }
    list = [
      {
        name = "a1"
        map = {
          a1 = 1
          b1 = 1
        }
      },
      {
        name = "a2"
      }
    ]
  }

  data_2 = {
    root = {
      elem2 = "value2"
      child1 = {
        cc2 = 2
      }
    }
    list = [
      {
        name = "a1"
        map = {
          a2 = 2
        }
      },
      {
        name = "a3"
      }
    ]
  }
}

# Merge data structures with list item merging enabled (default behavior)
output "merged_with_list_merging" {
  value = provider::utils::merge([local.data_1, local.data_2], true)
}

# Merge data structures with list item merging disabled
output "merged_without_list_merging" {
  value = provider::utils::merge([local.data_1, local.data_2], false)
}

/*
merged_with_list_merging = {
  "list" = [
    {
      "map" = {
        "a1" = 1
        "a2" = 2
        "b1" = 1
      }
      "name" = "a1"
    },
    {
      "name" = "a2"
    },
    {
      "name" = "a3"
    },
  ]
  "root" = {
    "child1" = {
      "cc1" = 1
      "cc2" = 2
    }
    "elem1" = "value1"
    "elem2" = "value2"
  }
}

merged_without_list_merging = {
  "list" = [
    {
      "map" = {
        "a1" = 1
        "b1" = 1
      }
      "name" = "a1"
    },
    {
      "name" = "a2"
    },
    {
      "map" = {
        "a2" = 2
      }
      "name" = "a1"
    },
    {
      "name" = "a3"
    },
  ]
  "root" = {
    "child1" = {
      "cc1" = 1
      "cc2" = 2
    }
    "elem1" = "value1"
    "elem2" = "value2"
  }
}
*/