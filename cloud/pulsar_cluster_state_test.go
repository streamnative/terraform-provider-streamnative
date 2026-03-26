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
	resourceData.Set("pool_member_name", "")

	cluster := &cloudv1alpha1.PulsarCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-a",
			Namespace: "org-a",
		},
		Spec: cloudv1alpha1.PulsarClusterSpec{
			InstanceName: "instance-a",
			Location:     "us-central1",
			PoolMemberRef: cloudv1alpha1.PoolMemberReference{
				Name:      "pool-member-a",
				Namespace: "org-a",
			},
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
	resourceData.Set("location", "")
	resourceData.Set("pool_member_name", "stale-pool-member")

	cluster := &cloudv1alpha1.PulsarCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-b",
			Namespace: "org-b",
		},
		Spec: cloudv1alpha1.PulsarClusterSpec{
			InstanceName: "instance-b",
			Location:     "us-central1",
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

func TestSetPulsarClusterIdentityStateImportHosted(t *testing.T) {
	resourceData := resourcePulsarCluster().TestResourceData()
	resourceData.Set("organization", "")
	resourceData.Set("name", "")
	resourceData.Set("instance_name", "")
	resourceData.Set("location", "")
	resourceData.Set("pool_member_name", "")

	cluster := &cloudv1alpha1.PulsarCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-c",
			Namespace: "org-c",
		},
		Spec: cloudv1alpha1.PulsarClusterSpec{
			InstanceName: "instance-c",
			Location:     "europe-west1",
			PoolMemberRef: cloudv1alpha1.PoolMemberReference{
				Name:      "pool-member-c",
				Namespace: "org-c",
			},
		},
	}

	diagErr := setPulsarClusterIdentityState(resourceData, cluster)
	assert.Nil(t, diagErr)
	assert.Equal(t, "org-c", resourceData.Get("organization"))
	assert.Equal(t, "cluster-c", resourceData.Get("name"))
	assert.Equal(t, "instance-c", resourceData.Get("instance_name"))
	assert.Equal(t, "europe-west1", resourceData.Get("location"))
	assert.Equal(t, "", resourceData.Get("pool_member_name"))
}

func TestSetPulsarClusterIdentityStateImportBYOC(t *testing.T) {
	resourceData := resourcePulsarCluster().TestResourceData()
	resourceData.Set("organization", "")
	resourceData.Set("name", "")
	resourceData.Set("instance_name", "")
	resourceData.Set("location", "")
	resourceData.Set("pool_member_name", "")

	cluster := &cloudv1alpha1.PulsarCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-d",
			Namespace: "org-d",
		},
		Spec: cloudv1alpha1.PulsarClusterSpec{
			InstanceName: "instance-d",
			PoolMemberRef: cloudv1alpha1.PoolMemberReference{
				Name:      "pool-member-d",
				Namespace: "org-d",
			},
		},
	}

	diagErr := setPulsarClusterIdentityState(resourceData, cluster)
	assert.Nil(t, diagErr)
	assert.Equal(t, "org-d", resourceData.Get("organization"))
	assert.Equal(t, "cluster-d", resourceData.Get("name"))
	assert.Equal(t, "instance-d", resourceData.Get("instance_name"))
	assert.Equal(t, "", resourceData.Get("location"))
	assert.Equal(t, "pool-member-d", resourceData.Get("pool_member_name"))
}
