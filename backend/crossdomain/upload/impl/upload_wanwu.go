package impl

import (
	"context"
	"fmt"

	"github.com/coze-dev/coze-studio/backend/api/model/app/developer_api"
	"github.com/coze-dev/coze-studio/backend/crossdomain/upload"
	"github.com/coze-dev/coze-studio/backend/infra/storage"
	"github.com/google/uuid"
)

type wanwuUploader struct {
	oss storage.Storage
}

func NewWanwuUploader(oss storage.Storage) upload.WanwuUploader {
	return &wanwuUploader{
		oss: oss,
	}
}

func (s *wanwuUploader) UploadFileByByte(ctx context.Context, fileName string, data []byte) (*upload.UploadFileResp, error) {
	var BizType developer_api.FileBizType = 0
	objectName := fmt.Sprintf("%s/%s/%s", BizType.String(), uuid.New().String(), fileName)

	err := s.oss.PutObject(ctx, objectName, data)
	if err != nil {
		return nil, err
	}

	url, err := s.oss.GetObjectUrl(ctx, objectName)
	if err != nil {
		return nil, err
	}

	return &upload.UploadFileResp{
		URL: url,
		URI: objectName,
	}, nil
}
