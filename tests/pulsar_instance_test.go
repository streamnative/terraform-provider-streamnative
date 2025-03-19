package test

import (
	"os"
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/stretchr/testify/assert"
)

var org = "sndev"
var ready = "True"
var availabilityMode = "zonal"
var poolName = "shared-gcp"
var poolNamespace = "streamnative"

func TestPulsarInstance(t *testing.T) {
	t.Parallel()
	// Construct the terraform options with default retryable errors to handle the most common
	// retryable errors in terraform testing.
	terraformOptions := terraform.WithDefaultRetryableErrors(t, &terraform.Options{
		// Set the path to the Terraform code that will be tested.
		TerraformDir: "tf-tests/pulsar_instance",
	})

	skipDestroy := os.Getenv("SKIP_DESTROY")

	if skipDestroy != "TRUE" {
		// Clean up resources with "terraform destroy" at the end of the test.
		defer terraform.Destroy(t, terraformOptions)
	}

	// Run "terraform apply". Fail the test if there are any errors.
	terraform.Apply(t, terraformOptions)

	// Run `terraform output` to get the values of output variables and check they have the expected values.
	resourceAvailabilityMode := terraform.Output(t, terraformOptions, "resource_availability_mode")
	resourceOrganization := terraform.Output(t, terraformOptions, "resource_organization")
	resourcePoolName := terraform.Output(t, terraformOptions, "resource_pool_name")
	resourcePoolNamespace := terraform.Output(t, terraformOptions, "resource_pool_namespace")
	resourceReady := terraform.Output(t, terraformOptions, "resource_ready")

	dataAvailabilityMode := terraform.Output(t, terraformOptions, "data_availability_mode")
	dataOrganization := terraform.Output(t, terraformOptions, "data_organization")
	dataPoolName := terraform.Output(t, terraformOptions, "data_pool_name")
	dataPoolNamespace := terraform.Output(t, terraformOptions, "data_pool_namespace")
	dataReady := terraform.Output(t, terraformOptions, "data_ready")

	assert.Equal(t, availabilityMode, resourceAvailabilityMode)
	assert.Equal(t, org, resourceOrganization)
	assert.Equal(t, poolName, resourcePoolName)
	assert.Equal(t, poolNamespace, resourcePoolNamespace)
	assert.Equal(t, ready, resourceReady)

	assert.Equal(t, availabilityMode, dataAvailabilityMode)
	assert.Equal(t, org, dataOrganization)
	assert.Equal(t, poolName, dataPoolName)
	assert.Equal(t, poolNamespace, dataPoolNamespace)
	assert.Equal(t, ready, dataReady)
}
