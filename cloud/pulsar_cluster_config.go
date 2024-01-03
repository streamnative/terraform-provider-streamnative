package cloud

import (
	cloudv1alpha1 "github.com/streamnative/cloud-api-server/pkg/apis/cloud/v1alpha1"
)

func flattenPulsarClusterConfig(in *cloudv1alpha1.Config) []interface{} {
	att := make(map[string]interface{})
	if in.WebsocketEnabled != nil {
		att["websocket_enabled"] = in.WebsocketEnabled
	}
	if in.FunctionEnabled != nil {
		att["function_enabled"] = in.FunctionEnabled
	}
	if in.TransactionEnabled != nil {
		att["transaction_enabled"] = in.TransactionEnabled
	}

	if in.Protocols != nil {
		att["protocols"] = flattenProtocols(in.Protocols)
	}
	if in.AuditLog != nil {
		att["audit_log"] = flattenAuditLog(in.AuditLog)
	}
	if in.Custom != nil {
		att["custom"] = in.Custom
	}

	return []interface{}{att}
}

func flattenProtocols(in *cloudv1alpha1.ProtocolsConfig) []interface{} {
	att := make(map[string]interface{})
	if in.Kafka != nil {
		att["kafka"] = flattenKafkaConfig()
	}
	if in.Mqtt != nil {
		att["mqtt"] = flattenMqttConfig()
	}
	return []interface{}{att}
}

func flattenKafkaConfig() map[string]interface{} {
	att := make(map[string]interface{})
	att["enabled"] = interface{}(true)
	return att
}

func flattenMqttConfig() map[string]interface{} {
	att := make(map[string]interface{})
	att["enabled"] = interface{}(true)
	return att
}

func flattenAuditLog(in *cloudv1alpha1.AuditLog) []interface{} {
	att := make(map[string]interface{})
	if in.Categories != nil {
		att["categories"] = flattenCategories(in.Categories)
	}
	return []interface{}{att}
}

func flattenCategories(in []string) []interface{} {
	var att []interface{}
	for _, category := range in {
		att = append(att, category)
	}
	return att
}
