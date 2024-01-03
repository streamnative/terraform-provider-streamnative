terraform {
  required_providers {
    streamnative = {
      version = "0.1.0"
      source = "streamnative/streamnative"
    }
  }
}

provider "streamnative" {
  # Please replace path use your own key file path
  key_file_path = "/Users/tuteng/Downloads/sndev-terraform-ci-test.json"
}

resource "streamnative_pulsar_cluster" "test-cluster-1" {
  organization = "sndev"
  name = "test-cluster-2"
  instance_name = "test-instance"
  location = "us-central1"
  bookie_replicas = 3
  broker_replicas = 2
  compute_unit = 0.3
  storage_unit = 0.3

  config {
    websocket_enabled = true
    function_enabled = false
    transaction_enabled = false
    protocols {
      mqtt = {
        enabled = false
      }
      kafka = {
        enabled = true
      }
    }
    audit_log {
      categories = ["Management", "Describe", "Produce", "Consume"]
    }
    custom = {
        allowAutoTopicCreation = "true"
    }
  }
}

output "pulsar_cluster" {
  value = streamnative_pulsar_cluster.test-cluster-1
}