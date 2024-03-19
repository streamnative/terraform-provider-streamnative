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
  key_file_path = "<PATH>"
}

resource "streamnative_cloud_connection" "test-cloud-connection" {
    organization = "streamnative"
    name = "aws-connection"
    type = "aws"
    aws {
        account_id = "1234567890"
    }
}

resource "streamnative_cloud_environment" "test-cloud-environment" {
	organization = "streamnative"
	name = "aws-cloud-environment"
	region = "us-west1"
	cloud_connection_name = "aws-connection"
	network {
		cidr = "10.0.0.0/10"
	}
}
data "streamnative_cloud_connection" "test-cloud-connection" {
  depends_on = [streamnative_cloud_connection.test-cloud-connection]
  name = streamnative_cloud_connection.test-cloud-connection.name
  organization = streamnative_cloud_connection.test-cloud-connection.organization
}
