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
  key_file_path = "/path/to/your/service/account/key.json"
}

resource "streamnative_apikey" "test-admin-a" {
  organization = "sndev"
  name = "test-admin-i"
  instance_name = "terraform-test-api-key-pulsar-instance"
  service_account_name = "test-tf-admin"
  description = "This is a test api key for terraform"
  customized_metadata = {
   "client_id": "abc"
  }
  # If you want to revoke the api key, you can set revoke to true
  # By default, after revoking an apikey object, all connections using that apikey will
  # fail after 1 minute due to an authentication exception.
  revoke = false
  # expiration_time = "2025-01-01T10:00:00Z"
  # If you don't want to set expiration time, you can set expiration_time to "0"
   expiration_time = "0"
}

data "streamnative_apikey" "test-admin-a" {
  depends_on = [streamnative_apikey.test-admin-a]
  organization = streamnative_apikey.test-admin-a.organization
  name = streamnative_apikey.test-admin-a.name
  private_key = streamnative_apikey.test-admin-a.private_key
}

output "apikey" {
  sensitive = true
  value = data.streamnative_apikey.test-admin-a
}
