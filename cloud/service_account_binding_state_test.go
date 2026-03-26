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

	"github.com/streamnative/cloud-api-server/pkg/apis/cloud/v1alpha1"
)

func TestIsServiceAccountBindingReadyWithIAMAccount(t *testing.T) {
	serviceAccountBinding := &v1alpha1.ServiceAccountBinding{
		Spec: v1alpha1.ServiceAccountBindingSpec{
			EnableIAMAccountCreation: true,
		},
		Status: v1alpha1.ServiceAccountBindingStatus{
			Conditions: []v1alpha1.Condition{
				{
					Type:   "IAMAccountReady",
					Status: "True",
				},
				{
					Type:   "Ready",
					Status: "True",
				},
			},
		},
	}

	if !isServiceAccountBindingReady(serviceAccountBinding) {
		t.Fatal("expected service account binding with IAM account creation enabled to be ready")
	}
}

func TestIsServiceAccountBindingReadyWithoutIAMAccount(t *testing.T) {
	serviceAccountBinding := &v1alpha1.ServiceAccountBinding{
		Spec: v1alpha1.ServiceAccountBindingSpec{
			EnableIAMAccountCreation: false,
		},
		Status: v1alpha1.ServiceAccountBindingStatus{
			Conditions: []v1alpha1.Condition{
				{
					Type:   "Ready",
					Status: "True",
				},
			},
		},
	}

	if !isServiceAccountBindingReady(serviceAccountBinding) {
		t.Fatal("expected service account binding without IAM account creation to be ready when Ready condition is true")
	}
}

func TestIsServiceAccountBindingReadyMissingIAMCondition(t *testing.T) {
	serviceAccountBinding := &v1alpha1.ServiceAccountBinding{
		Spec: v1alpha1.ServiceAccountBindingSpec{
			EnableIAMAccountCreation: true,
		},
		Status: v1alpha1.ServiceAccountBindingStatus{
			Conditions: []v1alpha1.Condition{
				{
					Type:   "Ready",
					Status: "True",
				},
			},
		},
	}

	if isServiceAccountBindingReady(serviceAccountBinding) {
		t.Fatal("expected service account binding to stay not ready until IAMAccountReady is true")
	}
}
