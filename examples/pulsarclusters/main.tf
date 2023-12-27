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
}

output "pulsar_cluster" {
  value = streamnative_pulsar_cluster.test-cluster-1
}