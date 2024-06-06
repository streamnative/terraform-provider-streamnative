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
      source  = "streamnative/streamnative"
    }
  }
}

provider "streamnative" {
  # Please replace path use your own key file path
  key_file_path = "/path/to/your/service/account/key.json"
}

resource "streamnative_pulsar_cluster" "test-cluster-1" {
  organization    = "sndev"
  name            = "test-cluster-1"
  instance_name   = "test-instance"
  location        = "us-central1"
  release_channel = "rapid"
  bookie_replicas = 3
  broker_replicas = 2
  compute_unit    = 0.3
  storage_unit    = 0.3

  config {
    websocket_enabled   = true
    function_enabled    = false
    transaction_enabled = false
    protocols {
      mqtt = {
        enabled = "false"
      }
      kafka = {
        enabled = "true"
      }
    }
    audit_log {
      categories = ["Management", "Describe", "Produce", "Consume"]
    }
    custom = {
      allowAutoTopicCreation = "true"
    }
  }
}

data "streamnative_pulsar_cluster" "test-cluster-1" {
  depends_on   = [streamnative_pulsar_cluster.test-cluster-1]
  organization = streamnative_pulsar_cluster.test-cluster-1.organization
  name         = streamnative_pulsar_cluster.test-cluster-1.name
}

output "pulsar_cluster_hosted" {
  value = data.streamnative_pulsar_cluster.test-cluster-1
}