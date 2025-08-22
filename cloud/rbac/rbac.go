package util

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	cloudv1alpha1 "github.com/streamnative/cloud-api-server/pkg/apis/cloud/v1alpha1"
)

type iteratorProcessor func(reflect.Value, string) error

func iterateStructWithProcessor(s reflect.Value, prefix string, processor iteratorProcessor) error {
	if s.Kind() != reflect.Struct {
		return fmt.Errorf("expected a struct reflect.Value, got %s", s.Kind())
	}
	sType := s.Type()
	for i := 0; i < sType.NumField(); i++ {
		field := sType.Field(i)
		fieldValue := s.Field(i)
		flagName := strings.ToLower(field.Name)
		if jsonTag := field.Tag.Get("json"); jsonTag != "" {
			parts := strings.Split(jsonTag, ",")
			if parts[0] != "" {
				flagName = parts[0]
			}
		}
		fullFlagName := prefix + flagName
		if fieldValue.Kind() == reflect.Ptr {
			if fieldValue.IsNil() {
				fieldValue.Set(reflect.New(fieldValue.Type().Elem()))
			}
			pointedToValue := fieldValue.Elem()
			switch pointedToValue.Kind() {
			case reflect.String:
				break
			case reflect.Struct:
				if err := iterateStructWithProcessor(pointedToValue, fullFlagName+"_", processor); err != nil {
					return err
				}
			default:
				return fmt.Errorf("unsupported pointer element type for field %s: %s", strings.ToLower(fullFlagName), pointedToValue.Kind())
			}
			if err := processor(fieldValue, strings.ToLower(fullFlagName)); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("unsupported field type (not a pointer) for %s: %s", strings.ToLower(fullFlagName), fieldValue.Kind())
		}
	}
	return nil
}

func ParseToResourceNameRestriction(rawData map[string]interface{}) (*cloudv1alpha1.ResourceNameRestriction, bool) {
	restriction := &cloudv1alpha1.ResourceNameRestriction{}
	updated := false
	if err := iterateStructWithProcessor(reflect.ValueOf(restriction).Elem(), "", func(fieldValue reflect.Value, fullName string) error {
		if value, exist := rawData[fullName]; exist {
			if reflect.TypeOf(value).Kind() == reflect.String {
				updated = true
				fieldValue.SetString(value.(string))
			}
		}
		return nil
	}); err != nil {
		panic(err)
	}
	return restriction, updated
}

func ParseToRaw(restriction *cloudv1alpha1.ResourceNameRestriction) (map[string]string, bool) {
	m := make(map[string]string)
	updated := false
	if err := iterateStructWithProcessor(reflect.ValueOf(restriction).Elem(), "", func(fieldValue reflect.Value, fullName string) error {
		pointedToValue := fieldValue.Elem()
		if pointedToValue.Kind() == reflect.String {
			updated = true
			m[fullName] = fieldValue.String()
		}
		return nil
	}); err != nil {
		panic(err)
	}
	return m, updated
}

func GenerateResourceRoleBinding() map[string]*schema.Schema {
	schemas := make(map[string]*schema.Schema)
	restriction := &cloudv1alpha1.ResourceNameRestriction{}
	if err := iterateStructWithProcessor(reflect.ValueOf(restriction).Elem(), "", func(fieldValue reflect.Value, fullName string) error {
		pointedToValue := fieldValue.Elem()
		if pointedToValue.Kind() == reflect.String {
			schemas[fullName] = &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			}
		}
		return nil
	}); err != nil {
		panic(err)
	}
	return schemas
}

func GenerateDataRoleBinding() map[string]*schema.Schema {
	schemas := make(map[string]*schema.Schema)
	restriction := &cloudv1alpha1.ResourceNameRestriction{}
	if err := iterateStructWithProcessor(reflect.ValueOf(restriction).Elem(), "", func(fieldValue reflect.Value, fullName string) error {
		pointedToValue := fieldValue.Elem()
		if pointedToValue.Kind() == reflect.String {
			schemas[fullName] = &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			}
		}
		return nil
	}); err != nil {
		panic(err)
	}
	return schemas
}
