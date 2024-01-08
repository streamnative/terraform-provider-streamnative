package cloud

import (
	sncloudv1 "github.com/tuteng/sncloud-go-sdk"
)

func getFactoryFromMeta(meta interface{}) *sncloudv1.APIClient {
	return meta.(*sncloudv1.APIClient)
}
