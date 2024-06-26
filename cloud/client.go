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

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	cloudv1alpha1 "github.com/streamnative/cloud-api-server/pkg/apis/cloud/v1alpha1"
	cloudclient "github.com/streamnative/cloud-api-server/pkg/client/clientset_generated/clientset"
)

func init() {
	if err := cloudv1alpha1.AddToScheme(scheme.Scheme); err != nil {
		panic(err)
	}
}

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
		return nil, fmt.Errorf("ClientSet NewForConfig: %v", err)
	}
	return clientSet, nil
}

func getDynamicClient(factory cmdutil.Factory) (dynamic.Interface, error) {
	config, err := factory.ToRESTConfig()
	if err != nil {
		return nil, fmt.Errorf("ToRESTConfig: %v", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("DynamicClient NewForConfig: %v", err)
	}
	return dynamicClient, nil
}
