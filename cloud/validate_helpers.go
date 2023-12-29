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
