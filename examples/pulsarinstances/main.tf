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

resource "streamnative_pulsar_instance" "test-instance-1" {
  organization = "sndev"
  name = "test-instance-1"
  availability_mode = "zonal"
  pool_name = "shared-gcp"
  pool_namespace = "streamnative"
}

data "streamnative_pulsar_instance" "test-instance-1" {
  depends_on = [streamnative_pulsar_instance.test-instance-1]
  name = streamnative_pulsar_instance.test-instance-1.name
  organization = streamnative_pulsar_instance.test-instance-1.organization
}

output "pulsar_instance" {
  value = data.streamnative_pulsar_instance.test-instance-1
}