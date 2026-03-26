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
	"testing"

	cloudv1alpha1 "github.com/streamnative/cloud-api-server/pkg/apis/cloud/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSetPulsarClusterIdentityStateHosted(t *testing.T) {
	resourceData := resourcePulsarCluster().TestResourceData()
	resourceData.Set("organization", "stale-org")
	resourceData.Set("name", "stale-name")
	resourceData.Set("instance_name", "stale-instance")
	resourceData.Set("location", "stale-location")
	resourceData.Set("pool_member_name", "stale-pool-member")

	cluster := &cloudv1alpha1.PulsarCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-a",
			Namespace: "org-a",
		},
		Spec: cloudv1alpha1.PulsarClusterSpec{
			InstanceName: "instance-a",
			Location:     "us-central1",
		},
	}

	diagErr := setPulsarClusterIdentityState(resourceData, cluster)
	assert.Nil(t, diagErr)
	assert.Equal(t, "org-a", resourceData.Get("organization"))
	assert.Equal(t, "cluster-a", resourceData.Get("name"))
	assert.Equal(t, "instance-a", resourceData.Get("instance_name"))
	assert.Equal(t, "us-central1", resourceData.Get("location"))
	assert.Equal(t, "", resourceData.Get("pool_member_name"))
}

func TestSetPulsarClusterIdentityStateBYOC(t *testing.T) {
	resourceData := resourcePulsarCluster().TestResourceData()
	resourceData.Set("organization", "stale-org")
	resourceData.Set("name", "stale-name")
	resourceData.Set("instance_name", "stale-instance")
	resourceData.Set("location", "stale-location")
	resourceData.Set("pool_member_name", "stale-pool-member")

	cluster := &cloudv1alpha1.PulsarCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-b",
			Namespace: "org-b",
		},
		Spec: cloudv1alpha1.PulsarClusterSpec{
			InstanceName: "instance-b",
			PoolMemberRef: cloudv1alpha1.PoolMemberReference{
				Name:      "pool-member-b",
				Namespace: "org-b",
			},
		},
	}

	diagErr := setPulsarClusterIdentityState(resourceData, cluster)
	assert.Nil(t, diagErr)
	assert.Equal(t, "org-b", resourceData.Get("organization"))
	assert.Equal(t, "cluster-b", resourceData.Get("name"))
	assert.Equal(t, "instance-b", resourceData.Get("instance_name"))
	assert.Equal(t, "", resourceData.Get("location"))
	assert.Equal(t, "pool-member-b", resourceData.Get("pool_member_name"))
}
