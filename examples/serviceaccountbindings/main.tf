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

resource "streamnative_service_account_binding" "func-runner" {
  organization = "sndev"
  service_account_name = "func-runner"
  cluster_name = "cluster"
}

data "streamnative_service_account_binding" "func-runner" {
  depends_on = [streamnative_service_account_binding.func-runner]
  organization = streamnative_service_account_binding.func-runner.organization
  name = streamnative_service_account_binding.func-runner.name
}

output "service_account_binding" {
  value = data.streamnative_service_account_binding.func-runner
}
