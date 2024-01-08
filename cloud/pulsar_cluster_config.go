package cloud

import (
	sncloudv1 "github.com/tuteng/sncloud-go-sdk"
)

func flattenPulsarClusterConfig(in *sncloudv1.V1alpha1Config) []interface{} {
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
		att["custom"] = *in.Custom
	}

	return []interface{}{att}
}

func flattenProtocols(in *sncloudv1.V1alpha1ProtocolsConfig) []interface{} {
	att := make(map[string]interface{})
	if in.Kafka != nil {
		att["kafka"] = flattenKafkaConfig("true")
	} else {
		att["kafka"] = flattenKafkaConfig("false")
	}
	if in.Mqtt != nil {
		att["mqtt"] = flattenMqttConfig("true")
	} else {
		att["mqtt"] = flattenMqttConfig("false")
	}
	return []interface{}{att}
}

func flattenKafkaConfig(flag string) map[string]interface{} {
	return map[string]interface{}{"enabled": flag}
}

func flattenMqttConfig(flag string) map[string]interface{} {
	return map[string]interface{}{"enabled": flag}
}

func flattenAuditLog(in *sncloudv1.V1alpha1AuditLog) []interface{} {
	att := make(map[string]interface{})
	if in.Categories != nil {
		att["categories"] = flattenCategories(in.Categories)
	}
	return []interface{}{att}
}

func flattenCategories(in []string) []interface{} {
	att := make([]interface{}, len(in))
	for i, category := range in {
		att[i] = category
	}
	return att
}
