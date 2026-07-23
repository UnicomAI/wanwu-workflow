/*
 * author wangliang
 */

package wanwu_fileparser

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/coze-dev/coze-studio/backend/domain/workflow/entity"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/entity/vo"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/internal/canvas/convert"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/internal/nodes"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/internal/schema"
	http_client "github.com/coze-dev/coze-studio/backend/pkg/http-client"
	"github.com/coze-dev/coze-studio/backend/pkg/sonic"
)

type WanWuRetrieveConfig struct {
}

func (r *WanWuRetrieveConfig) Adapt(_ context.Context, n *vo.Node, _ ...nodes.AdaptOption) (*schema.NodeSchema, error) {
	ns := &schema.NodeSchema{
		Key:     vo.NodeKey(n.ID),
		Type:    entity.NodeTypeWanWuFileParser,
		Name:    n.Data.Meta.Title,
		Configs: r,
	}

	if err := convert.SetInputsForNodeSchema(n, ns); err != nil {
		return nil, err
	}

	if err := convert.SetOutputTypesForNodeSchema(n, ns); err != nil {
		return nil, err
	}

	return ns, nil
}

func (r *WanWuRetrieveConfig) Build(_ context.Context, _ *schema.NodeSchema, _ ...schema.BuildOption) (any, error) {
	return &WanWuRetrieve{}, nil
}

type WanWuRetrieve struct {
}

// FileParserReq 对应 callback /v1/doc_parse 接口的请求体
type FileParserReq struct {
	FileUrl   string `json:"upload_file_url"`
	MaxToken  int    `json:"max_token"`
}

// FileParserResp 对应 callback /v1/doc_parse 接口的响应体
type FileParserResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data string `json:"data"`
}

func (kr *WanWuRetrieve) Invoke(ctx context.Context, input map[string]any) (map[string]any, error) {
	fileUrl, ok := input["FileUrl"].(string)
	if !ok {
		return nil, errors.New("capital query key is required")
	}
	//执行文件解析
	parser, err := CommonFileParser(ctx, fileUrl)
	if err != nil {
		return nil, err
	}
	result := map[string]any{
		"text": parser,
	}

	return result, nil
}

func CommonFileParser(ctx context.Context, fileUrl string) (string, error) {
	req := &FileParserReq{
		FileUrl:  fileUrl,
		MaxToken: 0, // 0 表示不截断，返回全文
	}

	resp, err := fileParser(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.Data, nil
}

// fileParser 文档解析，调用 callback 的 /v1/doc_parse 接口
func fileParser(ctx context.Context, req *FileParserReq) (*FileParserResp, error) {
	paramsByte, err := sonic.Marshal(req)
	if err != nil {
		return nil, err
	}
	result, err := http_client.GetDefaultClient().PostJson(ctx, &http_client.HttpRequestParams{
		Url:        os.Getenv("WANWU_FILE_PARSER_URL"),
		Body:       paramsByte,
		Timeout:    time.Duration(600) * time.Second, // 10 minutes
		MonitorKey: "file_parser",
		LogLevel:   http_client.LogAll,
	})
	if err != nil {
		return nil, err
	}
	var resp FileParserResp
	if err := sonic.Unmarshal(result, &resp); err != nil {
		return nil, err
	}
	if resp.Code != 0 {
		return nil, errors.New("file_parser error: " + resp.Msg)
	}
	if resp.Data == "" {
		return nil, errors.New("file_parser error: empty content")
	}
	return &resp, nil
}
