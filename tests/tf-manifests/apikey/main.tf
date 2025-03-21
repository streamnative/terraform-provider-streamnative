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
  name = "terraform-apikey-test-instance"
  availability_mode = "zonal"
  pool_name = "shared-gcp"
  pool_namespace = "streamnative"
  type = "standard"
}

resource "streamnative_pulsar_cluster" "test-cluster" {
  organization    = streamnative_pulsar_instance.test-instance.organization
  name            = "tfpc-apik"
  instance_name   = streamnative_pulsar_instance.test-instance.name
  location        = "us-central1"
  release_channel = "rapid"
  bookie_replicas = 3
  broker_replicas = 2
  compute_unit    = 0.3
  storage_unit    = 0.3
}

resource "streamnative_service_account" "test-service-account" {
  name = "terraform-apikey-test-service-account"
  organization = streamnative_pulsar_cluster.test-cluster.organization
}

resource "streamnative_apikey" "test-api-key" {
  depends_on = [streamnative_pulsar_cluster.test-cluster]
  organization = streamnative_pulsar_instance.test-instance.organization
  name = "terraform-apikey-test-key"
  instance_name = streamnative_pulsar_instance.test-instance.name
  service_account_name = streamnative_service_account.test-service-account.name
  # just for testing, please don't set it to true for avoid token revoked
  revoke = true
  description = "This is a test api key"
}

data "streamnative_apikey" "test-api-key" {
  name = "terraform-apikey-test-key"
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