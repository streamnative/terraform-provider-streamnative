terraform {
  required_providers {
    streamnative = {
      version = "0.1.0"
      source = "streamnative/streamnative"
    }
  }
}

provider "streamnative" {
  # Replace with your own client_id
  client_id = "Uxfu8OTq3uwuGJeIliACwEkEmBxhdDH5"
  # Please replace path use your own key file path
  key_file_path = "/Users/tuteng/Downloads/sndev-terraform-ci-test.json"
}

resource "streamnative_pulsar_instance" "test-instance-1" {
  organization = "sndev"
  name = "test-instance-1"
  availability_mode = "zonal"
  pool_name = "shared-gcp"
  pool_namespace = "streamnative"
}

output "pulsar_instance" {
  value = streamnative_pulsar_instance.test-instance-1
}