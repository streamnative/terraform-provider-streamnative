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
      source  = "streamnative/streamnative"
    }
  }
}

provider "streamnative" {
  # Please replace path use your own key file path
  key_file_path = "/path/to/your/service/account/key.json"
}

resource "streamnative_rolebinding" "basic_role_binding_2" {
  organization      = "o-y8z75"
  name              = "basic_role_binding_2"
  cluster_role_name = "org-readonly"
  user_names = ["user-1"]
}

resource "streamnative_rolebinding" "tenant_admin_role_binding" {
  organization      = "o-y8z75"
  name              = "tenant_admin_role_binding"
  cluster_role_name = "tenant-admin"
  user_names = ["user-2"]
  resource_name_restriction {
    common_instance = "instance-1"
    common_cluster  = "cluster-1"
    common_tenant   = "tenant-1"
  }
}

resource "streamnative_rolebinding" "topic_producer_role_binding" {
  name              = "topic_producer_role_binding"
  organization      = "o-y8z75"
  cluster_role_name = "topic-producer"
  service_account_names = ["serviceaccount-3"]
  resource_name_restriction {
    common_instance     = "instance-1"
    common_cluster      = "cluster-1"
    common_tenant       = "tenant-1"
    common_namespace    = "ns-1"
    common_topic        = "tp-1"
  }
}

resource "streamnative_rolebinding" "rb_resource_name_restriction" {
  name         = "rb_resource_name_restriction"
  organization = "o-y8z75"
  cluster_role_name = "topic-producer"
  service_account_names = ["sv-1"]
  resource_name_restriction {
    common_instance = "instance-1"
    common_cluster = "cluster-1"
    common_tenant = "tenant-1"
    common_namespace = "ns-1"
    common_topic = "allPartition('topic-1')"
    pulsar_topic_domain = "persistent"
  }
}

data "streamnative_rolebinding" "rb_resource_name_restriction" {
  depends_on = [streamnative_rolebinding.rb_resource_name_restriction]
  name         = "rb_resource_name_restriction"
  organization = "o-y8z75"
}


data "streamnative_rolebinding" "basic_role_binding" {
  depends_on = [streamnative_rolebinding.basic_role_binding_2]
  organization = streamnative_rolebinding.basic_role_binding_2.organization
  name         = streamnative_rolebinding.basic_role_binding_2.name
}

data "streamnative_rolebinding" "tenant_admin_role_binding" {
  depends_on = [streamnative_rolebinding.tenant_admin_role_binding]
  organization = streamnative_rolebinding.tenant_admin_role_binding.organization
  name         = streamnative_rolebinding.tenant_admin_role_binding.name
}

data "streamnative_rolebinding" "topic_producer_role_binding" {
  depends_on = [streamnative_rolebinding.topic_producer_role_binding]
  organization = streamnative_rolebinding.topic_producer_role_binding.organization
  name         = streamnative_rolebinding.topic_producer_role_binding.name
}

output "streamnative_rolebindings" {
  value = [
    data.streamnative_rolebinding.basic_role_binding,
    data.streamnative_rolebinding.tenant_admin_role_binding,
    data.streamnative_rolebinding.topic_producer_role_binding,
    data.streamnative_rolebinding.rb_resource_name_restriction
  ]
}