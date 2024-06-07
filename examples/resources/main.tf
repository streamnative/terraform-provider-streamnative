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

data "streamnative_resources" "pulsarclusters" {
  organization = "sndev"
  resource = "pulsarclusters"
}

data "streamnative_pulsar_cluster" "pulsarclusters" {
  count = length(data.streamnative_resources.pulsarclusters.names)
  organization = "sndev"
  name = data.streamnative_resources.pulsarclusters.names[count.index]
}

output "pulsarclusters" {
  value = data.streamnative_pulsar_cluster.pulsarclusters
}

data "streamnative_resources" "pulsarinstances" {
  organization = "sndev"
  resource = "pulsarinstances"
}

data "streamnative_pulsar_instance" "pulsarinstances" {
  count = length(data.streamnative_resources.pulsarinstances.names)
  organization = "sndev"
  name = data.streamnative_resources.pulsarinstances.names[count.index]
}

output "pulsarinstances" {
  value = data.streamnative_pulsar_instance.pulsarinstances
}