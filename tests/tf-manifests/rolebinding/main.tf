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

resource "streamnative_rolebinding" "rolebinding_demo" {
  organization = "sndev"
  name         = "terraform-test-rolebinding"
  cluster_role_name = "metrics-viewer"
  service_account_names = ["terraform-test-rolebinding"]
}

data "streamnative_rolebinding" "rolebinding_demo" {
  organization = streamnative_rolebinding.rolebinding_demo.organization
  name         = streamnative_rolebinding.rolebinding_demo.name
}

output "resource_rolebinding" {
  value = streamnative_rolebinding.rolebinding_demo
}

output "data_rolebinding" {
  value = data.streamnative_rolebinding.rolebinding_demo
}