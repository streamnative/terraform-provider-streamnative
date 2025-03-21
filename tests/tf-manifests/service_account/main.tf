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

resource "streamnative_service_account" "test-service-account" {
	organization = "sndev"
	name = "terraform-test-service-account"
	admin = true
}

data "streamnative_service_account" "test-service-account" {
  organization = streamnative_service_account.test-service-account.organization
  name = streamnative_service_account.test-service-account.name
}