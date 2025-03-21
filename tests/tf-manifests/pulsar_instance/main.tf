terraform {
  required_providers {
    streamnative = {
      source = "terraform.local/local/streamnative"
    }
  }
}

provider "streamnative" {
  # Please replace path use your own key file path
  key_file_path = pathexpand("~/service_account.json")
}

locals {
  rand = replace(substr(timestamp(), 11, 8), ":", "")
}

resource "streamnative_pulsar_instance" "test-instance" {
  organization = "sndev"
  name = "terraform-pulsar-instance-test-${local.rand}"
  availability_mode = "zonal"
  pool_name = "shared-gcp"
  pool_namespace = "streamnative"
  type = "standard"

  lifecycle {
    ignore_changes = [
      name,
    ]
  }
}

data "streamnative_pulsar_instance" "test-instance" {
  depends_on = [streamnative_pulsar_instance.test-instance]
  name = streamnative_pulsar_instance.test-instance.name
  organization = streamnative_pulsar_instance.test-instance.organization
}

output "resource_pulsar_instance" {
  value = streamnative_pulsar_instance.test-instance
}

output "data_pulsar_instance" {
  value = data.streamnative_pulsar_instance.test-instance
}