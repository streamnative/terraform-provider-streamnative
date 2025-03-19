package test

import (
	"os"
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/stretchr/testify/assert"
)

func TestPulsarCluster(t *testing.T) {
	t.Parallel()
	// Construct the terraform options with default retryable errors to handle the most common
	// retryable errors in terraform testing.
	terraformOptions := terraform.WithDefaultRetryableErrors(t, &terraform.Options{
		// Set the path to the Terraform code that will be tested.
		TerraformDir: "tf-tests/pulsar_cluster",
	})

	skipDestroy := os.Getenv("SKIP_DESTROY")

	if skipDestroy != "TRUE" {
		// Clean up resources with "terraform destroy" at the end of the test.
		defer terraform.Destroy(t, terraformOptions)
	}

	// Run "terraform init" and "terraform apply". Fail the test if there are any errors.
	terraform.Apply(t, terraformOptions)

	// Run `terraform output` to get the values of output variables and check they have the expected values.
	resourceBookieReplicas := terraform.Output(t, terraformOptions, "resource_bookie_replicas")

	assert.Equal(t, "3", resourceBookieReplicas)
}
