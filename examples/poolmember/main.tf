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

data "streamnative_resources" "poolmembers" {
  organization = "sndev"
  resource = "poolmembers"
}

data "streamnative_pool_member" "poolmembers" {
  count = length(data.streamnative_resources.poolmembers.names)
  organization = "sndev"
  name = data.streamnative_resources.poolmembers.names[count.index]
}

output "poolmembers" {
  value = data.streamnative_pool_member.poolmembers
}