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

	cloudclient "github.com/streamnative/cloud-api-server/pkg/client/clientset_generated/clientset"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

func getFactoryFromMeta(meta interface{}) cmdutil.Factory {
	return meta.(cmdutil.Factory)
}

func getClientSet(factory cmdutil.Factory) (*cloudclient.Clientset, error) {
	config, err := factory.ToRESTConfig()
	if err != nil {
		return nil, fmt.Errorf("ToRESTConfig: %v", err)
	}
	clientSet, err := cloudclient.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("NewForConfig: %v", err)
	}
	return clientSet, nil
}
