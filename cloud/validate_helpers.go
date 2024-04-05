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
	"fmt"
	"strconv"
	"strings"
)

func validateNotBlank(val interface{}, key string) (warns []string, errs []error) {
	v := val.(string)
	if len(strings.Trim(strings.TrimSpace(v), "\"")) == 0 {
		errs = append(errs, fmt.Errorf("%q must not be empty", key))
	}
	return
}

func validateBookieReplicas(val interface{}, key string) (warns []string, errs []error) {
	v := val.(int)
	if v < 3 || v > 15 {
		errs = append(errs, fmt.Errorf(
			"%q should be greater than or equal to 3 and less than or equal to 15, got: %d", key, v))
	}
	return
}

func validateBrokerReplicas(val interface{}, key string) (warns []string, errs []error) {
	v := val.(int)
	if v < 1 || v > 15 {
		errs = append(errs, fmt.Errorf(
			"%q should be greater than or equal to 1 and less than or equal to 15, got: %d", key, v))
	}
	return
}

func validateCUSU(val interface{}, key string) (warns []string, errs []error) {
	v := val.(float64)
	if v < 0.2 || v > 8 {
		errs = append(errs, fmt.Errorf(
			"%q should be greater than or equal to 0.2 and less than or equal to 8, got: %f", key, v))
	}
	return
}

func validateAuditLog(val interface{}, key string) (warns []string, errs []error) {
	v := val.(string)
	if val != "Management" && val != "Describe" && val != "Produce" && val != "Consume" {
		errs = append(errs, fmt.Errorf(
			"%q should be none, managed or external, got: %s", key, v))
	}
	return
}

func validateCloudEnvionmentType(val interface{}, key string) (warns []string, errs []error) {
	v := val.(string)
	if v != "test" && v != "staging" && v != "production" {
		errs = append(errs, fmt.Errorf(
			"%q should be one of: test, staging or production, got: %s", key, v))
	}
	return
}

func validateRegion(val interface{}, key string) (warns []string, errs []error) {
	v := val.(string)
	if !contains(validRegions, v) {
		errs = append(errs, fmt.Errorf(
			"%q must be a valid region, got: %s", key, v))
	}
	return
}

func validateCidrRange(val interface{}, key string) (warns []string, errs []error) {
	v := val.(string)
	parts := strings.Split(v, "/")

	if len(parts) < 2 {
		//Not valid CIDR notation
		errs = append(errs, fmt.Errorf(
			"%q is not valid CIDR notation, must be X.X.X.X/X, got: %s", key, v))
	} else {
		prefixLength, err := strconv.Atoi(parts[1])

		if err != nil || (prefixLength < 16 || prefixLength > 28) {
			errs = append(errs, fmt.Errorf(
				"%q is not valid CIDR prefix length, must be between /16 and /28, got: %s", key, v))
		}
	}
	return
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

var validRegions = []string{
	//GCP
	"us-west1",
	"us-west2",
	"us-west3",
	"us-west4",
	"us-central1",
	"us-east1",
	"us-east4",
	"northamerica-northeast1",
	"southamerica-east1",
	"europe-west2",
	"europe-west1",
	"europe-west4",
	"europe-west6",
	"europe-west3",
	"europe-north1",
	"asia-south1",
	"asia-southeast1",
	"asia-southeast2",
	"asia-east2",
	"asia-east1",
	"asia-northeast1",
	"asia-northeast2",
	"australia-southeast1",
	"asia-northeast3",
	//AWS
	"us-east-2",
	"us-east-1",
	"us-west-1",
	"us-west-2",
	"af-south-1",
	"ap-east-1",
	"ap-south-2",
	"ap-southeast-3",
	"ap-southeast-4",
	"ap-south-1",
	"ap-northeast-3",
	"ap-northeast-2",
	"ap-southeast-1",
	"ap-southeast-2",
	"ap-northeast-1",
	"ca-central-1",
	"ca-west-1",
	"eu-central-1",
	"eu-west-1",
	"eu-west-2",
	"eu-south-1",
	"eu-west-3",
	"eu-south-2",
	"eu-north-1",
	"eu-central-2",
	"il-central-1",
	"me-south-1",
	"me-central-1",
	"sa-east-",
	"eastus",
	//Azure
	"eastus2",
	"southcentralus",
	"westus2",
	"westus3",
	"australiaeast",
	"southeastasia",
	"northeurope",
	"swedencentral",
	"uksouth",
	"westeurope",
	"centralus",
	"southafricanorth",
	"centralindia",
	"eastasia",
	"japaneast",
	"koreacentral",
	"canadacentral",
	"francecentral",
	"germanywestcentral",
	"norwayeast",
	"polandcentral",
	"switzerlandnorth",
	"uaenorth",
	"brazilsouth",
	"centraluseuap",
	"qatarcentral",
	"centralusstage",
	"eastusstage",
	"eastus2stage",
	"northcentralusstage",
	"southcentralusstage",
	"westusstage",
	"westus2stage",
	"asia",
	"asiapacific",
	"australia",
	"brazil",
	"canada",
	"europe",
	"france",
	"germany",
	"global",
	"india",
	"japan",
	"korea",
	"norway",
	"singapore",
	"southafrica",
	"switzerland",
	"uae",
	"uk",
	"unitedstates",
	"unitedstateseuap",
	"eastasiastage",
	"southeastasiastage",
	"brazilus",
	"eastusstg",
	"northcentralus",
	"westus",
	"jioindiawest",
	"eastus2euap",
	"southcentralusstg",
	"westcentralus",
	"southafricawest",
	"australiacentral",
	"australiacentral2",
	"australiasoutheast",
	"japanwest",
	"jioindiacentral",
	"koreasouth",
	"southindia",
	"westindia",
	"canadaeast",
	"francesouth",
	"germanynorth",
	"norwaywest",
	"switzerlandwest",
	"ukwest",
	"uaecentral",
	"brazilsoutheas",
}
