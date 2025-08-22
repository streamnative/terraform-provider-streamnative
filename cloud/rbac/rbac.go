package rbac

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	cloudv1alpha1 "github.com/streamnative/cloud-api-server/pkg/apis/cloud/v1alpha1"
)

const resourceNotSet = "__RESOURCE_UNSET__"

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
				elem := fieldValue.Type().Elem()
				if elem.Kind() == reflect.Struct {
					fieldValue.Set(reflect.New(fieldValue.Type().Elem()))
				}
			}
			switch fieldValue.Type().Elem().Kind() {
			case reflect.Struct:
				if err := iterateStructWithProcessor(fieldValue.Elem(), fullFlagName+"_", processor); err != nil {
					return err
				}
			case reflect.String:
				break
			default:
				return fmt.Errorf("expected a struct pointer to a struct pointer, got %s", fieldValue.Elem().Kind())
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
				vstr := value.(string)
				if vstr != resourceNotSet {
					updated = true
					if fieldValue.IsNil() {
						fieldValue.Set(reflect.New(fieldValue.Type().Elem()))
					}
					fieldValue.Elem().SetString(vstr)
				}
			}
		}
		pointedToValue := fieldValue.Elem()
		if pointedToValue.Kind() == reflect.Struct && pointedToValue.IsZero() {
			fieldValue.Set(reflect.Zero(fieldValue.Type()))
		}
		return nil
	}); err != nil {
		panic(err)
	}
	return restriction, updated
}

func ParseToRaw(restriction *cloudv1alpha1.ResourceNameRestriction) (map[string]interface{}, bool) {
	m := make(map[string]interface{})
	updated := false
	if err := iterateStructWithProcessor(reflect.ValueOf(restriction).Elem(), "", func(fieldValue reflect.Value, fullName string) error {
		if (fieldValue.Kind() == reflect.Ptr && !fieldValue.IsNil()) && fieldValue.Elem().Kind() == reflect.String {
			updated = true
			m[fullName] = fieldValue.Elem().String()
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
		pointedToValue := fieldValue.Type().Elem()
		if pointedToValue.Kind() == reflect.String {
			schemas[fullName] = &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Default:  resourceNotSet,
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
		pointedToValue := fieldValue.Type().Elem()
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
