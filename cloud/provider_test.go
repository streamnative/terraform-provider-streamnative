package cloud

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"os"
	"testing"
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
	if v := os.Getenv("CLIENT_ID"); v == "" {
		t.Fatal("CLIENT_ID must be set for acceptance tests")
	}
	if v := os.Getenv("KEY_FILE_PATH"); v == "" {
		t.Fatal("KEY_FILE_PATH must be set for acceptance tests")
	}
}
