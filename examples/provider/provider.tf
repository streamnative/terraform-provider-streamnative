terraform {
  required_providers {
    streamnative = {
      source  = "streamnative/streamnative"
    }
  }
}

provider "streamnative" {
  # Please replace path use your own key file path
  key_file_path = "<your-key-file-path>"
}