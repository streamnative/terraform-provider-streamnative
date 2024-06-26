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
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var (
	testAccProvider          *schema.Provider
	testAccProviderFactories map[string]func() (*schema.Provider, error)
)

func init() {
	testAccProvider = Provider()
	testAccProviderFactories = map[string]func() (*schema.Provider, error){
		"streamnative": func() (*schema.Provider, error) {
			return testAccProvider, nil
		},
	}
}

func TestProvider(t *testing.T) {
	if err := Provider().InternalValidate(); err != nil {
		t.Fatal("err: %w", err)
	}
}

func TestProvider_impl(t *testing.T) {
	var _ = Provider()
}

func testAccPreCheck(t *testing.T) {
	keyFilePath := os.Getenv("KEY_FILE_PATH")
	clientId := os.Getenv("GLOBAL_DEFAULT_CLIENT_ID")
	clientSecret := os.Getenv("GLOBAL_DEFAULT_CLIENT_SECRET")
	if keyFilePath == "" && clientId == "" && clientSecret == "" {
		t.Fatal("KEY_FILE_PATH or GLOBAL_DEFAULT_CLIENT_ID," +
			"GLOBAL_DEFAULT_CLIENT_SECRET must be set for acceptance tests")
	}
}
