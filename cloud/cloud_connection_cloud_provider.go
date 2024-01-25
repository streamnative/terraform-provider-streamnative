// Copyright 2024 StreamNative, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cloud

import (
	cloudv1alpha1 "github.com/streamnative/cloud-api-server/pkg/apis/cloud/v1alpha1"
)

func flattenCloudConnectionAws(in *cloudv1alpha1.AWSCloudConnection) []interface{} {
	att := make(map[string]interface{})
	if in.AccountId != "" {
		att["account_id"] = in.AccountId
	}

	return []interface{}{att}
}

func flattenCloudConnectionGcloud(in *cloudv1alpha1.GCloudConnection) []interface{} {
	att := make(map[string]interface{})
	if in.ProjectId != "" {
		att["project"] = in.ProjectId
	}

	return []interface{}{att}
}
