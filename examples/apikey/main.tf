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
      source = "streamnative/streamnative"
    }
  }
}

provider "streamnative" {
  # Please replace path use your own key file path
  key_file_path = "/Users/tuteng/Downloads/sndev-terraform-ci-test.json"
}

resource "streamnative_apikey" "test-admin-a" {
  organization = "sndev"
  name = "test-admin-d"
  instance_name = "test-apikey"
  service_account_name = "test-tf-admin"
  revoke = false
  description = "This is a test api key"
}

data "streamnative_apikey" "test-admin-a" {
  depends_on = [streamnative_apikey.test-admin-a]
  organization = streamnative_apikey.test-admin-a.organization
  name = streamnative_apikey.test-admin-a.name
  private_key = streamnative_apikey.test-admin-a.private_key
}

output "apikey" {
  value = data.streamnative_apikey.test-admin-a
}
