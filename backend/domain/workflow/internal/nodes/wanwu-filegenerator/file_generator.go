/*
 * author wangliang
 */

package wanwu_filegenerator

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
	"github.com/spf13/cast"
)

type WanWuRetrieveConfig struct {
	RetrieveParams *RetrieveParams
}

type RetrieveParams struct {
	FileType string `json:"fileType"` //txt/pdf/docx
}

func (r *WanWuRetrieveConfig) Adapt(_ context.Context, n *vo.Node, _ ...nodes.AdaptOption) (*schema.NodeSchema, error) {
	ns := &schema.NodeSchema{
		Key:     vo.NodeKey(n.ID),
		Type:    entity.NodeTypeWanWuFileGenerator,
		Name:    n.Data.Meta.Title,
		Configs: r,
	}

	inputs := n.Data.Inputs

	retrieveParams := &RetrieveParams{}

	var getDesignatedParamContent = func(name string) (any, bool) {
		for _, param := range inputs.InputParameters {
			if param.Name == name {
				return param.Input.Value.Content, true
			}
		}
		return nil, false
	}

	if content, ok := getDesignatedParamContent("fileType"); ok {
		fileType := cast.ToString(content)
		retrieveParams.FileType = fileType
	}

	r.RetrieveParams = retrieveParams

	if err := convert.SetInputsForNodeSchema(n, ns); err != nil {
		return nil, err
	}

	if err := convert.SetOutputTypesForNodeSchema(n, ns); err != nil {
		return nil, err
	}

	return ns, nil
}

func (r *WanWuRetrieveConfig) Build(_ context.Context, _ *schema.NodeSchema, _ ...schema.BuildOption) (any, error) {

	if r.RetrieveParams == nil {
		return nil, errors.New("retrieval params is required")
	}

	return &WanWuRetrieve{
		retrieveParams: r.RetrieveParams,
	}, nil
}

type WanWuRetrieve struct {
	retrieveParams *RetrieveParams
}

type FileGenerateParams struct {
	Title             string `json:"title"`
	FormattedMarkdown string `json:"formatted_markdown"`
	ToFormat          string `json:"to_format"`
}

type FileGenerateResp struct {
	DownloadLink string `json:"download_link"`
}

func (kr *WanWuRetrieve) Invoke(ctx context.Context, input map[string]any) (map[string]any, error) {
	title, ok := input["Title"].(string)
	content, ok := input["Content"].(string)
	if !ok {
		return nil, errors.New("capital query key is required")
	}
	retrieveParams := kr.retrieveParams
	req := &FileGenerateParams{
		Title:             title,
		FormattedMarkdown: content,
		ToFormat:          retrieveParams.FileType,
	}

	response, err := fileGenerate(ctx, req)
	if err != nil {
		return nil, err
	}
	result := map[string]any{
		"fileUrl": response.DownloadLink,
	}

	return result, nil
}

// fileGenerate 文档生成
func fileGenerate(ctx context.Context, fileGenerateParams *FileGenerateParams) (*FileGenerateResp, error) {
	var params = make(map[string]string)
	params["title"] = fileGenerateParams.Title
	params["formatted_markdown"] = fileGenerateParams.FormattedMarkdown
	params["to_format"] = fileGenerateParams.ToFormat

	result, err := http_client.GetDefaultClient().PostForm(ctx, &http_client.HttpRequestParams{
		Url:        os.Getenv("WANWU_FILE_GENERATOR_URL"),
		Params:     params,
		Timeout:    time.Duration(600) * time.Second, // 10 minutes
		MonitorKey: "file_generate",
		LogLevel:   http_client.LogAll,
	})
	if err != nil {
		return nil, err
	}
	var resp FileGenerateResp
	if err := sonic.Unmarshal(result, &resp); err != nil {
		//log.Errorf(err.Error())
		return nil, err
	}
	return &resp, nil
}
