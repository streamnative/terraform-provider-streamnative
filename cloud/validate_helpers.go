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
