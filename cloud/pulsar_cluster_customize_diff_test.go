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

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClearServerlessLakehouseStorageDiff(t *testing.T) {
	resource := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"lakehouse_storage_enabled": {
				Type:     schema.TypeBool,
				Optional: true,
				Computed: true,
			},
		},
		CustomizeDiff: func(ctx context.Context, diff *schema.ResourceDiff, meta interface{}) error {
			clearServerlessLakehouseStorageDiff(ctx, diff)
			return nil
		},
	}

	state := &terraform.InstanceState{
		ID: "org/cluster",
		Attributes: map[string]string{
			"id":                        "org/cluster",
			"lakehouse_storage_enabled": "true",
		},
	}
	config := terraform.NewResourceConfigRaw(map[string]interface{}{
		"lakehouse_storage_enabled": false,
	})

	instanceDiff, err := resource.SimpleDiff(context.Background(), state, config, nil)
	require.NoError(t, err)
	assert.Empty(t, instanceDiff.Attributes)
}
