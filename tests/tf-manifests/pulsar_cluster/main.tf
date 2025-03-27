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
  name = "terraform-pulsar-cluster-test-instance"
  availability_mode = "zonal"
  pool_name = "shared-gcp"
  pool_namespace = "streamnative"
  type = "standard"
}

resource "streamnative_pulsar_instance" "test-instance-no-config" {
  organization = "sndev"
  name = "terraform-pulsar-cluster-test-instance-no-config"
  availability_mode = "zonal"
  pool_name = "shared-gcp"
  pool_namespace = "streamnative"
  type = "standard"
}

resource "streamnative_pulsar_cluster" "test-cluster" {
  organization    = streamnative_pulsar_instance.test-instance.organization
  name            = "tfpc-test"
  instance_name   = streamnative_pulsar_instance.test-instance.name
  location        = "us-central1"
  release_channel = "rapid"
  bookie_replicas = 3
  broker_replicas = 2
  compute_unit    = 0.3
  storage_unit    = 0.3

  config {
		websocket_enabled = false
		function_enabled = true
		transaction_enabled = false
		protocols {
		  mqtt = {
			enabled = "true"
		  }
		  kafka = {
			enabled = "true"
		  }
		}
		custom = {
			"bookkeeper.journalSyncData" = "false"
			"managedLedgerOffloadAutoTriggerSizeThresholdBytes" = "0"
		}
	}
}

resource "streamnative_pulsar_cluster" "test-cluster-no-config" {
  organization    = streamnative_pulsar_instance.test-instance-no-config.organization
  name            = "tfpc-nconf"
  instance_name   = streamnative_pulsar_instance.test-instance-no-config.name
  location        = "us-central1"
  release_channel = "rapid"
  bookie_replicas = 3
  broker_replicas = 2
  compute_unit    = 0.3
  storage_unit    = 0.3
}

data "streamnative_pulsar_cluster" "test-cluster" {
  organization = "sndev"
  name = "tfpc-test"
}

data "streamnative_pulsar_cluster" "test-cluster-no-config" {
  organization = "sndev"
  name = "tfpc-nconf"
}

output "resource_pulsar_cluster" {
  value = streamnative_pulsar_cluster.test-cluster
}

output "resource_pulsar_cluster_no_config" {
  value = streamnative_pulsar_cluster.test-cluster-no-config
}

output "data_pulsar_cluster" {
  value = data.streamnative_pulsar_cluster.test-cluster
}

output "data_pulsar_cluster_no_config" {
  value = data.streamnative_pulsar_cluster.test-cluster-no-config
}