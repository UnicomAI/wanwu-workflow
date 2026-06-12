/*
 * author wangliang
 */

package wanwu_fileparser

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/coze-dev/coze-studio/backend/domain/workflow/entity"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/entity/vo"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/internal/canvas/convert"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/internal/nodes"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/internal/schema"
	http_client "github.com/coze-dev/coze-studio/backend/pkg/http-client"
	"github.com/coze-dev/coze-studio/backend/pkg/sonic"
)

const (
	segmentSize = 500
	overlapSize = 0.0
)

var parserChoices = []string{"text"}

var separators = []string{
	"\n\n",
	"\n",
	" ",
	",",
	"\u200b", // 零宽空格
	"\uff0c", // 全角逗号
	"\u3001", // 顿号
	"\uff0e", // 全角句号
	"\u3002", // 句号
	".",
	""}

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

type FileParserParams struct {
	FileUrl       string   `json:"url"`
	ParserChoices []string `json:"parser_choices"`
	Overlap       float32  `json:"overlap_size" `
	SegmentSize   int      `json:"sentence_size"`
	Separators    []string `json:"separators"`
}

type FileParserResp struct {
	Docs []*FileParserInfo `json:"docs"`
}

type FileParserInfo struct {
	Metadata *FileParserMeta `json:"metadata"`
	Text     string          `json:"text"`
}

type FileParserMeta struct {
	FileName string `json:"file_name"`
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
	req := &FileParserParams{
		FileUrl:       fileUrl,
		ParserChoices: parserChoices,
		Overlap:       overlapSize,
		SegmentSize:   segmentSize,
		Separators:    separators,
	}

	response, err := fileParser(ctx, req)
	if err != nil {
		return "", err
	}

	//循环拼接结果
	var resultText strings.Builder
	for _, resp := range response {
		resultText.WriteString(resp.Text)
	}
	return resultText.String(), nil
}

// fileParser 文档解析
func fileParser(ctx context.Context, fileParserParams *FileParserParams) ([]*FileParserInfo, error) {
	paramsByte, err := sonic.Marshal(fileParserParams)
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
		//log.Errorf(err.Error())
		return nil, err
	}
	if len(resp.Docs) == 0 {
		return nil, errors.New("file_parser error")
	}
	return resp.Docs, nil
}
