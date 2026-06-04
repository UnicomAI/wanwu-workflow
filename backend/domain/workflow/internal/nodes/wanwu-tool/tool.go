package wanwu_tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	openapi3_util "github.com/UnicomAI/wanwu/pkg/openapi3-util"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/entity"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/entity/vo"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/internal/canvas/convert"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/internal/nodes"
	wanwu_util "github.com/coze-dev/coze-studio/backend/domain/workflow/internal/nodes/wanwu-util"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/internal/schema"
	http_client "github.com/coze-dev/coze-studio/backend/pkg/http-client"
	"github.com/coze-dev/coze-studio/backend/pkg/logs"
	"github.com/coze-dev/coze-studio/backend/pkg/sonic"
)

type Config struct {
	APISchema string
	ActionID  string
	ApiAuth   *openapi3_util.Auth
}

func (c *Config) Adapt(ctx context.Context, n *vo.Node, _ ...nodes.AdaptOption) (*schema.NodeSchema, error) {
	var err error
	inputs := n.Data.Inputs
	toolInfo := inputs.WanwuToolParam
	c.APISchema, c.ApiAuth, err = toolRequest(ctx, toolInfo.ToolID, toolInfo.ToolType, toolInfo.ApiKey)
	if err != nil {
		return nil, fmt.Errorf("tool request err: %v", err)
	}
	c.ActionID = toolInfo.ActionID
	ns := &schema.NodeSchema{
		Key:     vo.NodeKey(n.ID),
		Type:    entity.NodeTypeWanWuMCPTool,
		Name:    n.Data.Meta.Title,
		Configs: c,
	}
	if err := convert.SetInputsForNodeSchema(n, ns); err != nil {
		return nil, err
	}
	if err := convert.SetOutputTypesForNodeSchema(n, ns); err != nil {
		return nil, err
	}
	return ns, nil
}

func (c *Config) Build(ctx context.Context, ns *schema.NodeSchema, _ ...schema.BuildOption) (any, error) {
	if c.APISchema == "" {
		return nil, fmt.Errorf("api schema empty ")
	}

	client, err := openapi3_util.NewClient(ctx, []byte(c.APISchema))
	if err != nil {
		return nil, fmt.Errorf("failed to create openapi client: %v", err)
	}

	tool := &WanWuToolInfo{
		client:   client,
		actionId: c.ActionID,
		apiAuth:  c.ApiAuth,
	}
	return tool, nil
}

type WanWuToolInfo struct {
	actionId string
	client   *openapi3_util.Client
	apiAuth  *openapi3_util.Auth
}

func (i *WanWuToolInfo) Invoke(ctx context.Context, in map[string]any) (map[string]any, error) {
	// 解析输入参数
	params := parseInputParams(in)

	// 设置认证头
	if i.apiAuth != nil {
		if params.HeaderParams == nil {
			params.HeaderParams = make(map[string]string)
		}
		if i.apiAuth != nil && i.apiAuth.Type != "" && i.apiAuth.Type != "none" && i.apiAuth.Value != "" {
			switch i.apiAuth.In {
			case "header":
				params.HeaderParams[i.apiAuth.Name] = i.apiAuth.Value
			case "query":
				params.QueryParams[i.apiAuth.Name] = i.apiAuth.Value
			}
		}
	}

	// 构建请求参数
	requestParams := &openapi3_util.RequestParams{
		PathParams:   params.PathParams,
		QueryParams:  params.QueryParams,
		HeaderParams: params.HeaderParams,
		BodyParams:   params.BodyParams,
	}

	jsonData, err := json.Marshal(requestParams)
	if err != nil {
		return nil, fmt.Errorf("params marshal err: %v", err)
	}
	logs.Debugf("workflow tool node invoke action(%v) params: %v", i.actionId, string(jsonData))

	result, err := i.client.DoRequestByOperationID(ctx, i.actionId, requestParams)
	if err != nil {
		return nil, fmt.Errorf("api request failed: %v", err)
	}

	// 转换结果为 map[string]any
	var ret map[string]any
	switch v := result.(type) {
	case map[string]any:
		ret = v
	case string:
		// 如果是字符串，尝试解析为 JSON
		if err := sonic.Unmarshal([]byte(v), &ret); err != nil {
			// 如果解析失败，作为普通字符串返回
			ret = map[string]any{"result": v}
		}
	default:
		// 其他类型直接包装
		ret = map[string]any{"result": v}
	}

	return ret, nil
}

func parseInputParams(input map[string]any) *openapi3_util.RequestParams {
	params := &openapi3_util.RequestParams{
		HeaderParams: make(map[string]string),
		PathParams:   make(map[string]interface{}),
		QueryParams:  make(map[string]interface{}),
		BodyParams:   make(map[string]interface{}),
	}

	for key, value := range input {
		// 按照 "-" 分割位置和参数名
		parts := strings.SplitN(key, "-", 2)
		if len(parts) != 2 {
			// 如果没有指定位置，默认放到 body
			params.BodyParams[key] = value
			continue
		}

		position := strings.ToLower(parts[0])
		paramName := parts[1]

		switch position {
		case "path":
			params.PathParams[paramName] = fmt.Sprintf("%v", value)
		case "query":
			params.QueryParams[paramName] = value
		case "header":
			params.HeaderParams[paramName] = fmt.Sprintf("%v", value)
		case "body":
			params.BodyParams[paramName] = value
		default:
			// 未知位置，默认放到 body
			params.BodyParams[key] = value
		}
	}
	return params
}

// toolRequest 获取工具的 schema 和认证信息
func toolRequest(ctx context.Context, toolId, toolType, userApiKey string) (string, *openapi3_util.Auth, error) {
	switch toolType {
	case "custom":
		url, err := url.JoinPath(os.Getenv("WANWU_CALLBACK_CUSTOM_TOOL_URL"))
		if err != nil {
			return "", nil, err
		}
		var res response
		var ret customToolDetail
		resp, err := http_client.GetRestyClientWithTimeout(time.Minute).R().SetContext(ctx).
			SetHeader("Content-Type", "application/json").
			SetHeader("Accept", "application/json").
			SetQueryParam("customToolId", toolId).
			SetResult(&res).Get(url)
		if err != nil {
			return "", nil, fmt.Errorf("request %v err: %v", url, err)
		}
		if resp.StatusCode() >= 300 {
			return "", nil, fmt.Errorf("request %v http status %v msg: %v", url, resp.StatusCode(), res.Msg)
		}
		marshal, err := sonic.Marshal(res.Data)
		if err != nil {
			return "", nil, fmt.Errorf("request %v marshal response body: %v", url, err)
		}
		if err = sonic.Unmarshal(marshal, &ret); err != nil {
			return "", nil, fmt.Errorf("request %v unmarshal response body: %v", url, err)
		}
		var apiAuth *openapi3_util.Auth
		apiAuth, err = ret.ApiAuth.ToOpenapiAuth()
		if err != nil {
			return "", nil, fmt.Errorf("request %v custom tool api auth to openapi auth err: %v", url, err)
		}
		return ret.Schema, apiAuth, nil
	case "builtin":
		url, err := url.JoinPath(os.Getenv("WANWU_CALLBACK_SQUARE_TOOL_URL"))
		if err != nil {
			return "", nil, err
		}
		var res response
		var ret toolSquareDetail
		resp, err := http_client.GetRestyClientWithTimeout(time.Minute).R().SetContext(ctx).
			SetHeader("Content-Type", "application/json").
			SetHeader("Accept", "application/json").
			SetQueryParam("toolSquareId", toolId).
			SetResult(&res).Get(url)
		if err != nil {
			return "", nil, fmt.Errorf("request %v err: %v", url, err)
		}
		if resp.StatusCode() >= 300 {
			return "", nil, fmt.Errorf("request %v http status %v msg: %v", url, resp.StatusCode(), res.Msg)
		}
		marshal, err := sonic.Marshal(res.Data)
		if err != nil {
			return "", nil, fmt.Errorf("request %v marshal response body: %v", url, err)
		}
		if err = sonic.Unmarshal(marshal, &ret); err != nil {
			return "", nil, fmt.Errorf("request %v unmarshal response body: %v", url, err)
		}
		var apiAuth *openapi3_util.Auth
		ret.ApiAuth.ApiKeyValue = userApiKey
		apiAuth, err = ret.ApiAuth.ToOpenapiAuth()
		if err != nil {
			return "", nil, fmt.Errorf("request %v builtin tool api auth to openapi auth err: %v", url, err)
		}
		return ret.Schema, apiAuth, nil
	}
	return "", nil, errors.New("unsupported tool type")
}

// 原有的结构体保持不变
type response struct {
	Code int64  `json:"code"`
	Data any    `json:"data"`
	Msg  string `json:"msg"`
}

type customToolApiResponse struct {
	Name   string `json:"name"`
	Method string `json:"method"`
	Path   string `json:"path"`
}

type customToolDetail struct {
	CustomToolId  string                       `json:"customToolId"`  // 自定义工具id
	Name          string                       `json:"name"`          // 名称
	Description   string                       `json:"description"`   // 描述
	Schema        string                       `json:"schema"`        // schema
	ApiAuth       wanwu_util.ApiAuthWebRequest `json:"apiAuth"`       // apiAuth
	ApiList       []customToolApiResponse      `json:"apiList"`       // api列表
	PrivacyPolicy string                       `json:"privacyPolicy"` // 隐私政策
}

type toolSquareInfo struct {
	ToolSquareID string `json:"toolSquareId"` // 广场mcpId(非空表示来源于广场)
	Name         string `json:"name"`         // 名称
	Desc         string `json:"desc"`         // 描述
}

type toolSquareActions struct {
	NeedApiKeyInput bool                         `json:"needApiKeyInput"` // 是否需要apiKey输入
	APIKey          string                       `json:"apiKey"`          // apiKey
	ApiAuth         wanwu_util.ApiAuthWebRequest `json:"apiAuth"`         // apiAuth
	Tools           []mcpTool                    `json:"tools"`           // 工具列表
	Detail          string                       `json:"detail"`          // 详细描述
	ActionSum       int64                        `json:"actionSum"`       // action总数
}

type mcpTool struct {
	Name        string             `json:"name"`        // 工具名
	Description string             `json:"description"` // 工具描述
	InputSchema mcpToolInputSchema `json:"inputSchema"` // 工具参数
}

type mcpToolInputSchema struct {
	Type       string                             `json:"type"`       // 固定值: object
	Properties map[string]mcpToolInputSchemaValue `json:"properties"` // 字段名 -> 字段信息
	Required   []string                           `json:"required"`   // 必填字段
}

type mcpToolInputSchemaValue struct {
	Type        string `json:"type"`        // 字段类型
	Description string `json:"description"` // 字段描述
}

type toolSquareDetail struct {
	toolSquareInfo
	toolSquareActions
	Schema string `json:"schema"`
}
