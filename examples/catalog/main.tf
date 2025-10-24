# Copyright 2024 StreamNative, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

terraform {
  required_providers {
    streamnative = {
      source = "streamnative/streamnative"
    }
  }
}

provider "streamnative" {
  # Please replace path use your own key file path
  key_file_path = "/path/to/your/service/account/key.json"
}

# S3 Table Catalog example (mode is optional, defaults to "EXTERNAL")
resource "streamnative_catalog" "s3_table_catalog" {
  organization = "max"
  name         = "s3-table-catalog-example"
  # mode is optional and defaults to "EXTERNAL"

  # S3Table configuration with ARN format (required)
  s3_table_bucket = "arn:aws:s3tables:ap-northeast-1:598203581484:bucket/s3-table-test"
}

# Data source example
data "streamnative_catalog" "existing_catalog" {
  organization = "max"
  name         = "s3-table-catalog-example"
}

# Output the catalog information
output "catalog_info" {
  value = {
    # S3Table
    s3_table_catalog_ready = streamnative_catalog.s3_table_catalog.ready
    s3_table_region        = streamnative_catalog.s3_table_catalog.s3_table_region
    
    # Data source
    existing_catalog_mode = data.streamnative_catalog.existing_catalog.mode
  }
}