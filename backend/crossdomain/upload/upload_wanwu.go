package upload

import (
	"context"
)

var defaultWanwuSVC WanwuUploader

type WanwuUploader interface {
	UploadFileByByte(ctx context.Context, fileName string, data []byte) (resp *UploadFileResp, err error)
}

func SetDefaultWanwuSVC(s WanwuUploader) {
	defaultWanwuSVC = s
}

func DefaultWanwuSVC() WanwuUploader {
	return defaultWanwuSVC
}

type UploadFileResp struct {
	URL string `thrift:"url,1" form:"url" json:"url" query:"url"`
	URI string `thrift:"uri,2" form:"uri" json:"uri" query:"uri"`
}
