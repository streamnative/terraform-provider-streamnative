package cloud

import (
	"github.com/streamnative/cloud-api-server/pkg/apis/cloud"
	cloudv1alpha1 "github.com/streamnative/cloud-api-server/pkg/apis/cloud/v1alpha1"
)

func flattenPrivateService(in *cloudv1alpha1.PrivateService) []interface{} {
	att := make(map[string]interface{})
	if in.AllowedIds != nil {
		att["allowed_ids"] = flattenStringSlice(in.AllowedIds)
	}
	return []interface{}{att}
}

func flattenPrivateServiceIds(in []cloudv1alpha1.PrivateServiceId) []interface{} {
	ids := make([]string, 0, len(in))
	for _, v := range in {
		ids = append(ids, v.Id)
	}
	return flattenStringSlice(ids)
}

func flattenStringSlice(in []string) []interface{} {
	ids := make([]interface{}, 0, len(in))
	for _, v := range in {
		ids = append(ids, v)
	}
	return ids
}

func flattenDefaultGateway(in *cloudv1alpha1.Gateway) []interface{} {
	att := make(map[string]interface{})
	if in.Access != "" {
		att["access"] = in.Access
	} else {
		att["access"] = "public"
	}
	if in.Access == "private" && in.PrivateService != nil {
		att["private_service"] = flattenPrivateService(in.PrivateService)
	}
	return []interface{}{att}
}

func convertGateway(val interface{}) *cloudv1alpha1.Gateway {
	gatewayRaw := val.([]interface{})
	if len(gatewayRaw) > 0 {
		defaultGatewayItemMap, ok := gatewayRaw[0].(map[string]interface{})
		var gateway cloudv1alpha1.Gateway
		access, ok := defaultGatewayItemMap["access"]
		if ok && access != "" {
			gateway = cloudv1alpha1.Gateway{
				Access: cloudv1alpha1.AccessType(defaultGatewayItemMap["access"].(string)),
			}
		}
		if access == string(cloud.PrivateAccess) {
			gateway.PrivateService = convertPrivateService(defaultGatewayItemMap["private_service"])
		}
		return &gateway
	}
	return nil
}

func convertPrivateService(val interface{}) *cloudv1alpha1.PrivateService {
	privateServiceRaw := val.([]interface{})
	var privateService cloudv1alpha1.PrivateService
	for _, privateServiceItem := range privateServiceRaw {
		privateServiceItemMap, ok := privateServiceItem.(map[string]interface{})
		if ok && privateServiceItemMap["allowed_ids"] != nil {
			allowedIdsRaw := privateServiceItemMap["allowed_ids"].([]interface{})
			allowedIds := make([]string, len(allowedIdsRaw))
			for i, v := range allowedIdsRaw {
				allowedIds[i] = v.(string)
			}
			privateService = cloudv1alpha1.PrivateService{
				AllowedIds: allowedIds,
			}
		}
	}
	return &privateService
}
