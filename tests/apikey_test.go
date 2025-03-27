package test

import (
	"os"
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/stretchr/testify/assert"
)

func TestApiKey(t *testing.T) {
	t.Parallel()
	// Construct the terraform options with default retryable errors to handle the most common
	// retryable errors in terraform testing.
	terraformOptions := terraform.WithDefaultRetryableErrors(t, &terraform.Options{
		// Set the path to the Terraform code that will be tested.
		TerraformDir: "tf-manifests/apikey",
	})

	skipDestroy := os.Getenv("SKIP_DESTROY")

	if skipDestroy != "TRUE" {
		// Clean up resources with "terraform destroy" at the end of the test.
		defer terraform.Destroy(t, terraformOptions)
	}

	// Run "terraform init" and "terraform apply". Fail the test if there are any errors.
	terraform.Apply(t, terraformOptions)

	// Run `terraform output` to get the values of output variables and check they have the expected values.
	resourceApiKey := terraform.OutputMap(t, terraformOptions, "resource_apikey")
	dataApiKey := terraform.OutputMap(t, terraformOptions, "data_apikey")

	assert.Equal(t, "terraform-apikey-test-key", resourceApiKey["name"])
	assert.Equal(t, "sndev/terraform-apikey-test-key", resourceApiKey["id"])
	assert.Equal(t, "This is a test api key", resourceApiKey["description"])
	assert.Equal(t, "terraform-apikey-test-instance", resourceApiKey["instance_name"])
	assert.Equal(t, "True", resourceApiKey["ready"])
	assert.Equal(t, "terraform-apikey-test-service-account", resourceApiKey["service_account_name"])
	assert.NotEmpty(t, resourceApiKey["expires_at"])
	assert.NotEmpty(t, resourceApiKey["issued_at"])
	assert.NotEmpty(t, resourceApiKey["key_id"])
	assert.NotEmpty(t, resourceApiKey["token"])

	assert.Equal(t, "terraform-apikey-test-key", dataApiKey["name"])
	assert.Equal(t, "sndev/terraform-apikey-test-key", dataApiKey["id"])
	assert.Equal(t, "This is a test api key", dataApiKey["description"])
	assert.Equal(t, "terraform-apikey-test-instance", dataApiKey["instance_name"])
	assert.Equal(t, "True", dataApiKey["ready"])
	assert.Equal(t, "terraform-apikey-test-service-account", dataApiKey["service_account_name"])
	assert.NotEmpty(t, dataApiKey["expires_at"])
	assert.NotEmpty(t, dataApiKey["issued_at"])
	assert.NotEmpty(t, dataApiKey["key_id"])
	assert.NotEmpty(t, dataApiKey["token"])
}
