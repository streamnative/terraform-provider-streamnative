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
)

func Test_validateSubnetCIDR(t *testing.T) {
	tests := []struct {
		subnet    string
		parent    string
		expect    bool
		expectErr bool
	}{
		// Valid subnets
		{"10.0.1.0/24", "10.0.0.0/16", true, false},
		{"192.168.1.0/25", "192.168.1.0/24", true, false},
		{"172.16.5.128/26", "172.16.0.0/16", true, false},

		// Subnet not contained in parent
		{"192.168.2.0/24", "192.168.1.0/24", false, false},
		{"10.1.0.0/16", "10.0.0.0/16", false, false},

		// Subnet broader than parent (mask too small)
		{"10.0.0.0/8", "10.0.0.0/16", false, false},
		{"172.16.0.0/12", "172.16.0.0/16", false, false},

		// Equal subnet and parent
		{"192.168.1.0/24", "192.168.1.0/24", true, false},

		// Invalid input
		{"invalid", "192.168.1.0/24", false, true},
		{"192.168.1.0/24", "invalid", false, true},
	}

	for _, tt := range tests {
		ok, err := validateSubnetCIDR(tt.subnet, tt.parent)
		if tt.expectErr {
			if err == nil {
				t.Errorf("Expected error for input (%s, %s), got none", tt.subnet, tt.parent)
			}
			continue
		}
		if err != nil {
			t.Errorf("Unexpected error for input (%s, %s): %v", tt.subnet, tt.parent, err)
			continue
		}
		if ok != tt.expect {
			t.Errorf("For (%s, %s), expected %v, got %v", tt.subnet, tt.parent, tt.expect, ok)
		}
	}
}
