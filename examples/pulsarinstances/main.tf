# // Licensed to the Apache Software Foundation (ASF) under one
# // or more contributor license agreements.  See the NOTICE file
# // distributed with this work for additional information
# // regarding copyright ownership.  The ASF licenses this file
# // to you under the Apache License, Version 2.0 (the
# // "License"); you may not use this file except in compliance
# // with the License.  You may obtain a copy of the License at
# //
# //   http://www.apache.org/licenses/LICENSE-2.0
# //
# // Unless required by applicable law or agreed to in writing,
# // software distributed under the License is distributed on an
# // "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# // KIND, either express or implied.  See the License for the
# // specific language governing permissions and limitations
# // under the License.

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