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
	"context"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
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

func TestSetPulsarClusterDataSourceIdentityState(t *testing.T) {
	resourceData := dataSourcePulsarCluster().TestResourceData()

	cluster := &cloudv1alpha1.PulsarCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-e",
			Namespace: "org-e",
		},
		Spec: cloudv1alpha1.PulsarClusterSpec{
			Location: "us-central1",
		},
	}
	instance := &cloudv1alpha1.PulsarInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "instance-e",
			Namespace: "org-e",
		},
	}

	diagErr := setPulsarClusterDataSourceIdentityState(resourceData, cluster, instance)
	assert.Nil(t, diagErr)
	assert.Equal(t, "instance-e", resourceData.Get("instance_name"))
	assert.Equal(t, "us-central1", resourceData.Get("location"))
}

func TestValidateMaintenanceWindowAccepted(t *testing.T) {
	expected := &cloudv1alpha1.MaintenanceWindow{
		Recurrence: "0,1",
		Window: &cloudv1alpha1.Window{
			StartTime: "02:00",
			Duration:  &metav1.Duration{Duration: 2 * time.Hour},
		},
	}

	err := validateMaintenanceWindowAccepted(expected, expected.DeepCopy(), "UPDATE")
	assert.NoError(t, err)
}

func TestValidateMaintenanceWindowAcceptedWhenDropped(t *testing.T) {
	expected := &cloudv1alpha1.MaintenanceWindow{
		Recurrence: "0,1",
	}

	err := validateMaintenanceWindowAccepted(expected, nil, "CREATE")
	assert.EqualError(t, err, "ERROR_CREATE_PULSAR_CLUSTER: maintenance_window is not enabled for this organization")
}

func TestExpandMaintenanceWindow(t *testing.T) {
	duration := 2 * time.Hour

	maintenanceWindow := expandMaintenanceWindow(context.Background(), []interface{}{
		map[string]interface{}{
			"recurrence": "0,1",
			"window": []interface{}{
				map[string]interface{}{
					"start_time": "02:00",
					"duration":   "2h0m0s",
				},
			},
		},
	})

	if assert.NotNil(t, maintenanceWindow) {
		assert.Equal(t, "0,1", maintenanceWindow.Recurrence)
		if assert.NotNil(t, maintenanceWindow.Window) {
			assert.Equal(t, "02:00", maintenanceWindow.Window.StartTime)
			if assert.NotNil(t, maintenanceWindow.Window.Duration) {
				assert.Equal(t, duration, maintenanceWindow.Window.Duration.Duration)
			}
		}
	}
}

func TestExpandMaintenanceWindowEmpty(t *testing.T) {
	assert.Nil(t, expandMaintenanceWindow(context.Background(), nil))
	assert.Nil(t, expandMaintenanceWindow(context.Background(), []interface{}{}))
}

func TestMaintenanceWindowSchemaStrictlyManaged(t *testing.T) {
	resourceSchema := resourcePulsarCluster().Schema
	maintenanceWindowSchema := resourceSchema["maintenance_window"]
	if assert.NotNil(t, maintenanceWindowSchema) {
		assert.True(t, maintenanceWindowSchema.Optional)
		assert.False(t, maintenanceWindowSchema.Computed)
	}

	maintenanceWindowResource := maintenanceWindowSchema.Elem.(*schema.Resource)
	windowSchema := maintenanceWindowResource.Schema["window"]
	if assert.NotNil(t, windowSchema) {
		assert.True(t, windowSchema.Optional)
		assert.False(t, windowSchema.Computed)
	}

	windowResource := windowSchema.Elem.(*schema.Resource)
	startTimeSchema := windowResource.Schema["start_time"]
	if assert.NotNil(t, startTimeSchema) {
		assert.True(t, startTimeSchema.Optional)
		assert.False(t, startTimeSchema.Computed)
	}

	durationSchema := windowResource.Schema["duration"]
	if assert.NotNil(t, durationSchema) {
		assert.True(t, durationSchema.Optional)
		assert.False(t, durationSchema.Computed)
	}

	recurrenceSchema := maintenanceWindowResource.Schema["recurrence"]
	if assert.NotNil(t, recurrenceSchema) {
		assert.True(t, recurrenceSchema.Optional)
		assert.False(t, recurrenceSchema.Computed)
	}
}

func TestMaintenanceWindowEqual(t *testing.T) {
	expected := &cloudv1alpha1.MaintenanceWindow{
		Recurrence: "0,1",
		Window: &cloudv1alpha1.Window{
			StartTime: "02:00",
			Duration:  &metav1.Duration{Duration: 2 * time.Hour},
		},
	}

	actual := &cloudv1alpha1.MaintenanceWindow{
		Recurrence: "0,1",
		Window: &cloudv1alpha1.Window{
			StartTime: "02:00",
			Duration:  &metav1.Duration{Duration: 2 * time.Hour},
		},
	}

	assert.True(t, maintenanceWindowEqual(expected, actual))
}

func TestMaintenanceWindowEqualWhenDifferent(t *testing.T) {
	expected := &cloudv1alpha1.MaintenanceWindow{
		Recurrence: "0,1",
	}
	actual := &cloudv1alpha1.MaintenanceWindow{
		Recurrence: "2,3",
	}

	assert.False(t, maintenanceWindowEqual(expected, actual))
}

func TestExpandBrokerAutoScalingPolicy(t *testing.T) {
	policy := expandBrokerAutoScalingPolicy([]interface{}{
		map[string]interface{}{
			"min_replicas": 2,
			"max_replicas": 6,
		},
	})

	assert.NotNil(t, policy)
	assert.Equal(t, int32(6), policy.MaxReplicas)
	assert.NotNil(t, policy.MinReplicas)
	assert.Equal(t, int32(2), *policy.MinReplicas)
}

// min_replicas is Computed, so an omitted value arrives as 0. Sending that literally would ask the
// control plane for a floor of no brokers, so it has to stay nil and be filled in server side.
func TestExpandBrokerAutoScalingPolicyOmittedMinReplicas(t *testing.T) {
	policy := expandBrokerAutoScalingPolicy([]interface{}{
		map[string]interface{}{
			"min_replicas": 0,
			"max_replicas": 4,
		},
	})

	assert.NotNil(t, policy)
	assert.Equal(t, int32(4), policy.MaxReplicas)
	assert.Nil(t, policy.MinReplicas)
}

func TestExpandBrokerAutoScalingPolicyEmpty(t *testing.T) {
	assert.Nil(t, expandBrokerAutoScalingPolicy(nil))
	assert.Nil(t, expandBrokerAutoScalingPolicy([]interface{}{}))
	assert.Nil(t, expandBrokerAutoScalingPolicy([]interface{}{nil}))
}

func TestFlattenBrokerAutoScalingPolicy(t *testing.T) {
	minReplicas := int32(3)
	flattened := flattenBrokerAutoScalingPolicy(&cloudv1alpha1.AutoScalingPolicy{
		MinReplicas: &minReplicas,
		MaxReplicas: 9,
	})

	assert.Len(t, flattened, 1)
	assert.Equal(t, map[string]interface{}{"min_replicas": 3, "max_replicas": 9}, flattened[0])
	assert.Equal(t, []interface{}{}, flattenBrokerAutoScalingPolicy(nil))
}

func TestBrokerAutoScalingPolicyEqual(t *testing.T) {
	minReplicas := int32(2)
	expected := &cloudv1alpha1.AutoScalingPolicy{MinReplicas: &minReplicas, MaxReplicas: 6}
	actualMin := int32(2)
	actual := &cloudv1alpha1.AutoScalingPolicy{MinReplicas: &actualMin, MaxReplicas: 6}

	assert.True(t, brokerAutoScalingPolicyEqual(expected, actual))
	assert.True(t, brokerAutoScalingPolicyEqual(nil, nil))
	assert.False(t, brokerAutoScalingPolicyEqual(expected, nil))
	assert.False(t, brokerAutoScalingPolicyEqual(nil, actual))
}

func TestBrokerAutoScalingPolicyEqualWhenDifferent(t *testing.T) {
	minReplicas := int32(2)
	expected := &cloudv1alpha1.AutoScalingPolicy{MinReplicas: &minReplicas, MaxReplicas: 6}

	differentMax := &cloudv1alpha1.AutoScalingPolicy{MinReplicas: &minReplicas, MaxReplicas: 7}
	assert.False(t, brokerAutoScalingPolicyEqual(expected, differentMax))

	otherMin := int32(3)
	differentMin := &cloudv1alpha1.AutoScalingPolicy{MinReplicas: &otherMin, MaxReplicas: 6}
	assert.False(t, brokerAutoScalingPolicyEqual(expected, differentMin))

	droppedMin := &cloudv1alpha1.AutoScalingPolicy{MaxReplicas: 6}
	assert.False(t, brokerAutoScalingPolicyEqual(expected, droppedMin))
}

// A min_replicas the user did not ask for is the control plane's to choose, so a server-filled value
// must not read as drift.
func TestBrokerAutoScalingPolicyEqualWhenMinReplicasDefaulted(t *testing.T) {
	expected := &cloudv1alpha1.AutoScalingPolicy{MaxReplicas: 6}
	serverFilled := int32(2)
	actual := &cloudv1alpha1.AutoScalingPolicy{MinReplicas: &serverFilled, MaxReplicas: 6}

	assert.True(t, brokerAutoScalingPolicyEqual(expected, actual))
}

func TestValidateBrokerAutoScalingPolicyAccepted(t *testing.T) {
	minReplicas := int32(2)
	expected := &cloudv1alpha1.AutoScalingPolicy{MinReplicas: &minReplicas, MaxReplicas: 6}

	assert.NoError(t, validateBrokerAutoScalingPolicyAccepted(expected, expected, "CREATE"))
}

func TestValidateBrokerAutoScalingPolicyAcceptedWhenDropped(t *testing.T) {
	expected := &cloudv1alpha1.AutoScalingPolicy{MaxReplicas: 6}

	err := validateBrokerAutoScalingPolicyAccepted(expected, nil, "CREATE")
	assert.EqualError(t, err,
		"ERROR_CREATE_PULSAR_CLUSTER: broker_auto_scaling_policy is not enabled for this organization")
}

// The control plane pins serverless clusters to its own policy; catching that as a mismatch is what
// stops a silently overridden configuration from being reported as applied.
func TestValidateBrokerAutoScalingPolicyAcceptedWhenOverridden(t *testing.T) {
	requestedMin := int32(1)
	expected := &cloudv1alpha1.AutoScalingPolicy{MinReplicas: &requestedMin, MaxReplicas: 10}
	pinnedMin := int32(2)
	actual := &cloudv1alpha1.AutoScalingPolicy{MinReplicas: &pinnedMin, MaxReplicas: 3}

	err := validateBrokerAutoScalingPolicyAccepted(expected, actual, "UPDATE")
	assert.EqualError(t, err,
		"ERROR_UPDATE_PULSAR_CLUSTER: broker_auto_scaling_policy is not enabled for this organization")
}
