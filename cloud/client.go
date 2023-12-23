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
