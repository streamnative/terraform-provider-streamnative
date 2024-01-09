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
