package rbac

import (
	"reflect"
	"testing"

	"github.com/streamnative/cloud-api-server/pkg/apis/cloud/v1alpha1"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"
)

func TestParser(t *testing.T) {
	restriction := &v1alpha1.ResourceNameRestriction{
		Common: &v1alpha1.CommonAttributes{
			Organization: ptr.To("org-1"),
			Instance:     ptr.To("ins-1"),
			Cluster:      ptr.To("cluster-1"),
			Tenant:       ptr.To("tenant-1"),
			Namespace:    ptr.To("namespace-1"),
			Topic:        ptr.To("topic-1"),
		},
		Pulsar: &v1alpha1.PulsarAttributes{
			Topic: &v1alpha1.PulsarTopicAttributes{
				Domain: ptr.To("domain-1"),
			},
			Subscription: &v1alpha1.PulsarSubscriptionAttributes{
				Name: ptr.To("subscription-1"),
			},
		},
		Cloud: &v1alpha1.CloudAttributes{
			Apikey: &v1alpha1.CloudApiKeyAttributes{
				Name: ptr.To("api-key-1"),
			},
		},
	}

	raw, updated := ParseToRaw(restriction)
	assert.True(t, updated)
	assert.NotNil(t, raw)
	parsedRestriction, updated := ParseToResourceNameRestriction(raw)
	assert.True(t, updated)
	assert.NotNil(t, parsedRestriction)
	assert.True(t, reflect.DeepEqual(restriction, parsedRestriction))
}

func TestParserIgnoreUnset(t *testing.T) {
	raw := map[string]interface{}{
		"common_organization":      "org-1",
		"common_instance":          "ins-1",
		"common_cluster":           "cluster-1",
		"cloud_apikey_name":        resourceNotSet,
		"pulsar_subscription_name": resourceNotSet,
	}
	parsedRestriction, updated := ParseToResourceNameRestriction(raw)
	assert.True(t, updated)
	assert.True(t, reflect.DeepEqual(&v1alpha1.ResourceNameRestriction{
		Common: &v1alpha1.CommonAttributes{
			Organization: ptr.To("org-1"),
			Instance:     ptr.To("ins-1"),
			Cluster:      ptr.To("cluster-1"),
		},
	}, parsedRestriction))
}
