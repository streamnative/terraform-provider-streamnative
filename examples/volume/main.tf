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
  key_file_path = "<your-key-file-path>"
}

resource "streamnative_volume" "test-volume" {
  organization = "max"
  name = "test-volume"
  bucket = "test-pulsar"
  path = "test-pulsar/data"
  region = "us-west-2"
  role_arn = "arn:aws:iam::123456789012:role/role-name"
}

data "streamnative_volume" "test-volume" {
  depends_on = [streamnative_volume.test-volume]
  organization = streamnative_volume.test-volume.organization
  name = streamnative_volume.test-volume.name
}
