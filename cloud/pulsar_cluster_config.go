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

func flattenAuditLog(in *cloudv1alpha1.AuditLog) []interface{} {
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

func flattenMaintenanceWindow(in *cloudv1alpha1.MaintenanceWindow) []interface{} {
	if in == nil {
		return []interface{}{}
	}

	att := make(map[string]interface{})

	if in.Window != nil {
		att["window"] = flattenWindow(in.Window)
	}
	if in.Recurrence != "" {
		att["recurrence"] = in.Recurrence
	}

	return []interface{}{att}
}

func flattenWindow(in *cloudv1alpha1.Window) []interface{} {
	if in == nil {
		return []interface{}{}
	}

	att := make(map[string]interface{})

	// Set StartTime if provided
	if in.StartTime != "" {
		att["start_time"] = in.StartTime
	}

	// Set Duration if provided
	if in.Duration != nil {
		att["duration"] = in.Duration.Duration.String()
	}

	return []interface{}{att}
}
