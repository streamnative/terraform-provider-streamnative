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
var poolName = "shared-gcp-prod"
var poolNamespace = "streamnative"

func TestPulsarInstance(t *testing.T) {
	t.Parallel()
	// Construct the terraform options with default retryable errors to handle the most common
	// retryable errors in terraform testing.
	terraformOptions := terraform.WithDefaultRetryableErrors(t, &terraform.Options{
		// Set the path to the Terraform code that will be tested.
		TerraformDir: "tf-manifests/pulsar_instance",
	})

	skipDestroy := os.Getenv("SKIP_DESTROY")

	if skipDestroy != "TRUE" {
		// Clean up resources with "terraform destroy" at the end of the test.
		defer terraform.Destroy(t, terraformOptions)
	}

	// Run "terraform apply". Fail the test if there are any errors.
	terraform.Apply(t, terraformOptions)

	// Run `terraform output` to get the values of output variables and check they have the expected values.
	resourcePulsarInstance := terraform.OutputMap(t, terraformOptions, "resource_pulsar_instance")
	dataPulsarInstance := terraform.OutputMap(t, terraformOptions, "data_pulsar_instance")

	assert.Equal(t, availabilityMode, resourcePulsarInstance["availability_mode"])
	assert.Equal(t, org, resourcePulsarInstance["organization"])
	assert.Equal(t, poolName, resourcePulsarInstance["pool_name"])
	assert.Equal(t, poolNamespace, resourcePulsarInstance["pool_namespace"])
	assert.Equal(t, ready, resourcePulsarInstance["ready"])

	assert.Equal(t, availabilityMode, dataPulsarInstance["availability_mode"])
	assert.Equal(t, org, dataPulsarInstance["organization"])
	assert.Equal(t, poolName, dataPulsarInstance["pool_name"])
	assert.Equal(t, poolNamespace, dataPulsarInstance["pool_namespace"])
	assert.Equal(t, ready, dataPulsarInstance["ready"])
}
