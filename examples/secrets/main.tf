// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

terraform {
  required_providers {
    streamnative = {
      version = "0.1.0"
      source  = "streamnative/streamnative"
    }
  }
}

provider "streamnative" {
  # Replace with your own key file path or client credentials
  key_file_path = "/path/to/your/service/account/key.json"
}

variable "cert_p12_path" {
  type        = string
  description = "Path to the local PKCS#12 certificate bundle to store in binary_data."
}

resource "streamnative_secret" "example" {
  organization  = "sndev"
  name          = "tf-secret"
  instance_name = "pulsar-instance-name"
  location      = "us-west2"
  string_data = {
    username = "demo-user"
    password = "demo-password"
  }

  binary_data = {
    "cert.p12" = filebase64(var.cert_p12_path)
  }
}

data "streamnative_secret" "example" {
  depends_on   = [streamnative_secret.example]
  organization = streamnative_secret.example.organization
  name         = streamnative_secret.example.name
}
