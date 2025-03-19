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

resource "streamnative_pulsar_instance" "test-instance" {
  organization = "sndev"
  name = "terraform-pulsar-instance-test"
  availability_mode = "zonal"
  pool_name = "shared-gcp"
  pool_namespace = "streamnative"
}

data "streamnative_pulsar_instance" "test-instance" {
  depends_on = [streamnative_pulsar_instance.test-instance]
  name = streamnative_pulsar_instance.test-instance.name
  organization = streamnative_pulsar_instance.test-instance.organization
}

output "resource_availability_mode" {
  value = streamnative_pulsar_instance.test-instance.availability_mode
}

output "resource_id" {
  value = streamnative_pulsar_instance.test-instance.id
}

output "resource_name" {
  value = streamnative_pulsar_instance.test-instance.name
}

output "resource_organization" {
  value = streamnative_pulsar_instance.test-instance.organization
}

output "resource_pool_name" {
  value = streamnative_pulsar_instance.test-instance.pool_name
}

output "resource_pool_namespace" {
  value = streamnative_pulsar_instance.test-instance.pool_namespace
}

output "resource_ready" {
  value = streamnative_pulsar_instance.test-instance.ready
}

output "data_availability_mode" {
  value = data.streamnative_pulsar_instance.test-instance.availability_mode
}

output "data_id" {
  value = data.streamnative_pulsar_instance.test-instance.id
}

output "data_name" {
  value = data.streamnative_pulsar_instance.test-instance.name
}

output "data_oauth2_audience" {
  value = data.streamnative_pulsar_instance.test-instance.oauth2_audience
}

output "data_oauth2_issuer_url" {
  value = data.streamnative_pulsar_instance.test-instance.oauth2_issuer_url
}

output "data_organization" {
  value = data.streamnative_pulsar_instance.test-instance.organization
}

output "data_pool_name" {
  value = data.streamnative_pulsar_instance.test-instance.pool_name
}

output "data_pool_namespace" {
  value = data.streamnative_pulsar_instance.test-instance.pool_namespace
}

output "data_ready" {
  value = data.streamnative_pulsar_instance.test-instance.ready
}