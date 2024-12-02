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

resource "streamnative_pulsar_instance" "test-ursa" {
  organization = "max"
  name = "test-ursa"
  availability_mode = "regional"
  pool_name = "shared-aws"
  pool_namespace = "max"
  type = "byoc"
  engine = "ursa"
}

data "streamnative_pulsar_instance" "test-ursa" {
  depends_on = [streamnative_pulsar_instance.test-ursa]
  name = streamnative_pulsar_instance.test-ursa.name
  organization = streamnative_pulsar_instance.test-ursa.organization
}

output "pulsar_instance" {
  value = data.streamnative_pulsar_instance.test-ursa
}

resource "streamnative_pulsar_cluster" "test-ursa" {
  depends_on = [streamnative_pulsar_instance.test-ursa]
  organization    = streamnative_pulsar_instance.test-ursa.organization
  name = "test-ursa"
  display_name            = "test-ursa"
  instance_name   = streamnative_pulsar_instance.test-ursa.name
  location        = "us-east-1"
  pool_member_name = "aws-use1-dev-rm6kq"
  release_channel = "rapid"
}

data "streamnative_pulsar_cluster" "test-ursa" {
  depends_on   = [streamnative_pulsar_cluster.test-ursa]
  organization = streamnative_pulsar_cluster.test-ursa.organization
  name         = streamnative_pulsar_cluster.test-ursa.name
}

output "pulsar_cluster_ursa" {
  value = data.streamnative_pulsar_cluster.test-ursa
}