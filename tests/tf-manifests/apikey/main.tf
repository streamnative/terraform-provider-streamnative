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
  name = "tf-apikey-test-instance-${local.rand}"
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

resource "streamnative_pulsar_cluster" "test-cluster" {
  organization    = streamnative_pulsar_instance.test-instance.organization
  name            = "tf-${local.rand}"
  instance_name   = streamnative_pulsar_instance.test-instance.name
  location        = "us-central1"
  release_channel = "rapid"
  bookie_replicas = 3
  broker_replicas = 2
  compute_unit    = 0.3
  storage_unit    = 0.3

  lifecycle {
    ignore_changes = [
      name,
    ]
  }
}

resource "streamnative_service_account" "test-service-account" {
  name = "tf-apikey-test-service-account-${local.rand}"
  organization = streamnative_pulsar_cluster.test-cluster.organization

  lifecycle {
    ignore_changes = [
      name,
    ]
  }
}

resource "streamnative_apikey" "test-api-key" {
  depends_on = [streamnative_pulsar_cluster.test-cluster]
  organization = streamnative_pulsar_instance.test-instance.organization
  name = "tf-apikey-test-key-${local.rand}"
  instance_name = streamnative_pulsar_instance.test-instance.name
  service_account_name = streamnative_service_account.test-service-account.name
  # just for testing, please don't set it to true for avoid token revoked
  revoke = true
  description = "This is a test api key"

  lifecycle {
    ignore_changes = [
      name,
    ]
  }
}

data "streamnative_apikey" "test-api-key" {
  name = "tf-apikey-test-key-${local.rand}"
  organization = streamnative_apikey.test-api-key.organization
}

output "resource_apikey" {
  value = streamnative_apikey.test-api-key
  sensitive = true
}

output "data_apikey" {
  value = data.streamnative_apikey.test-api-key
  sensitive = true
}

output "rand" {
  value = local.rand
}