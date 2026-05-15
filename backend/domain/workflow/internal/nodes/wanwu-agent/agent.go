package wanwu_agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	openapi3_util "github.com/UnicomAI/wanwu/pkg/openapi3-util"
	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/schema"
	"github.com/coze-dev/coze-studio/backend/application/base/ctxutil"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/entity"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/entity/vo"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/internal/canvas/convert"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/internal/nodes"
	wanwu_util "github.com/coze-dev/coze-studio/backend/domain/workflow/internal/nodes/wanwu-util"
	schema2 "github.com/coze-dev/coze-studio/backend/domain/workflow/internal/schema"
	"github.com/coze-dev/coze-studio/backend/pkg/logs"
	"github.com/coze-dev/coze-studio/backend/pkg/safego"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-resty/resty/v2"
	"github.com/spf13/cast"
)

const (
	WanWuAgentAPIUrlEnv           = "WANWU_AGENT_API_URL"
	WanWuCustomToolUrlEnv         = "WANWU_CALLBACK_CUSTOM_TOOL_URL"
	WanWuSquareToolUrlEnv         = "WANWU_CALLBACK_SQUARE_TOOL_URL"
	WanWuMCPGetUrlEnv             = "WANWU_CALLBACK_MCP_GET_URL"
	WanWuMCPServerGetUrlEnv       = "WANWU_CALLBACK_MCP_SERVER_GET_URL"
	WanWuWorkflowListSchemaUrlEnv = "WANWU_CALLBACK_WORKFLOW_LIST_SCHEMA_URL"
	AgentOutputKey                = "output"
	metaTypeNumber                = "number"
	metaTypeTime                  = "time"
	MCPTypeMCP                    = "mcp"
	MCPTypeMCPServer              = "mcpserver"
	ToolTypeBuiltIn               = "builtin" // 内置工具
	ToolTypeCustom                = "custom"  // 自定义工具
	MCPTransportSSE               = "sse"
	MCPTransportStreamable        = "streamable"
)

type Config struct {
	AgentBaseParams *AgentBaseParams
	LLMParams       *vo.LLMParams
	KnowledgeInfos  []*RetrieveKnowledgeInfo
	RetrieveParams  *RetrieveParams
	ToolParams      *ToolParams
}

type RetrieveParams struct {
	MatchType                   string  `json:"matchType"  validate:"required"` //matchType：vector（向量检索）、text（文本检索）、mix（混合检索：向量+文本）
	RerankModelId               string  `json:"rerankModelId"`                  //rerank模型id
	PriorityMatch               int     `json:"priorityMatch"`                  // 权重匹配，只有在混合检索模式下，选择权重设置后，这个才设置为1
	SemanticsPriority           float64 `json:"semanticsPriority"`              // 语义权重
	KeywordPriority             float64 `json:"keywordPriority"`                // 关键词权重
	RerankKeywordPriority       float64 `json:"rerankKeywordPriority"`          // 混合搜索的关键词权重
	RerankKeywordPrioritySwitch bool    `json:"rerankKeywordPrioritySwitch"`    // 混合搜索的关键词权重开关
	TopK                        int64   `json:"topK"`                           //topK 获取最高的几行
	Threshold                   float64 `json:"threshold"`                      //threshold 过滤分数阈值
	Rewrite                     bool    `json:"rewrite"`                        //是否开启重写
	UseGraph                    bool    `json:"useGraph"`                       // 是否开启知识图谱
}

type HitParams struct {
	UserId                string                 `json:"userId"`
	Question              string                 `json:"question" validate:"required"`
	KnowledgeIdList       []string               `json:"knowledgeIdList" validate:"required"`
	Threshold             float64                `json:"threshold"`
	TopK                  int64                  `json:"topK"`
	RerankModelId         string                 `json:"rerank_model_id"`               // rerankId
	RerankMod             string                 `json:"rerank_mod"`                    // rerank_model:重排序模式，weighted_score：权重搜索
	RetrieveMethod        string                 `json:"retrieve_method"`               // hybrid_search:混合搜索， semantic_search:向量搜索， full_text_search：文本搜索
	Weight                *WeightParams          `json:"weights"`                       // 权重搜索下的权重配置
	RewriteQuery          bool                   `json:"rewrite_query"`                 // 查询重写
	ReturnMeta            bool                   `json:"return_meta"`                   // 展示角标
	TermWeightCoefficient *float64               `json:"term_weight_coefficient"`       // 展示角标
	MetaFilter            bool                   `json:"metadata_filtering"`            // 元数据过滤开关
	MetaFilterConditions  []*MetadataFilterParam `json:"metadata_filtering_conditions"` // 元数据过滤条件
	UseGraph              bool                   `json:"use_graph"`                     // 知识图谱
}

type MetadataFilterParam struct {
	FilterKnowledgeName string                `json:"filtering_kb_name"`
	LogicalOperator     string                `json:"logical_operator"`
	MetaList            []*MetadataFilterItem `json:"conditions"` // 元数据过滤列表
}

type MetadataFilterItem struct {
	MetaName           string      `json:"meta_name"`           // 元数据名称
	MetaType           string      `json:"meta_type"`           // 元数据类型
	ComparisonOperator string      `json:"comparison_operator"` // 比较运算符
	Value              interface{} `json:"value,omitempty"`     // 用于过滤的条件值
}

type WeightParams struct {
	VectorWeight float64 `json:"vector_weight"` //语义权重
	TextWeight   float64 `json:"text_weight"`   //关键字权重
}

type RetrieveKnowledgeInfo struct {
	DatasetId            string                `json:"dataset_id"`
	Name                 string                `json:"name"`
	RagName              string                `json:"ragName"`
	KnowledgeId          string                `json:"knowledgeId"`
	MetaDataFilterParams *MetaDataFilterParams `json:"metaDataFilterParams"`
}

type MetaDataFilterParams struct {
	FilterEnable     bool                `json:"filterEnable"`
	FilterLogicType  string              `json:"filterLogicType"`
	MetaFilterParams []*MetaFilterParams `json:"metaFilterParams"`
}

type MetaFilterParams struct {
	Condition string `json:"condition"`
	Key       string `json:"key"`
	Type      string `json:"type"`
	Value     string `json:"value"`
}

func (c *Config) Adapt(ctx context.Context, n *vo.Node, _ ...nodes.AdaptOption) (*schema2.NodeSchema, error) {
	ns := &schema2.NodeSchema{
		Key:     vo.NodeKey(n.ID),
		Type:    entity.NodeTypeWanWuAgent,
		Name:    n.Data.Meta.Title,
		Configs: c,
	}

	inputs := n.Data.Inputs
	meta := n.Data.Meta

	if err := c.setLLMConfig(ctx, inputs, meta); err != nil {
		return nil, err
	}

	if err := c.setKnowledgeConfig(ctx, inputs); err != nil {
		return nil, err
	}

	if err := c.setToolConfig(ctx, inputs); err != nil {
		return nil, err
	}

	if err := convert.SetInputsForNodeSchema(n, ns); err != nil {
		return nil, err
	}

	if err := convert.SetOutputTypesForNodeSchema(n, ns); err != nil {
		return nil, err
	}

	// ns.StreamConfigs = &schema2.StreamConfig{
	// 	CanGeneratesStream: true, //声明agent节点支持流式输出
	// }

	cB, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal agent config: %w", err)
	}

	logs.CtxDebugf(ctx, "[AgentNode]  agent config: %s", string(cB))
	return ns, nil
}

func (c *Config) setLLMConfig(ctx context.Context, inputs *vo.Inputs, meta *vo.NodeMetaFE) error {
	param := inputs.LLMParam
	if param == nil {
		return fmt.Errorf("llm node's llmParam is nil")
	}

	bs, _ := sonic.Marshal(param)
	llmParam := make(vo.LLMParam, 0)
	if err := sonic.Unmarshal(bs, &llmParam); err != nil {
		return err
	}
	convertedLLMParam, err := llmParamsToLLMParam(llmParam)
	if err != nil {
		return err
	}

	c.LLMParams = convertedLLMParam

	logs.CtxDebugf(ctx, "[AgentNode.Adapt] Extracted config -  ModelID: %s, ModelName: %s",
		c.LLMParams.ModelType, c.LLMParams.ModelName)

	c.AgentBaseParams = &AgentBaseParams{
		Name:        meta.Title,
		Description: meta.Description,
		Instruction: convertedLLMParam.SystemPrompt,
	}

	return nil
}

func (c *Config) setKnowledgeConfig(_ context.Context, inputs *vo.Inputs) error {
	if len(inputs.DatasetParam) == 0 {
		return nil
	}
	datasetListInfoParam := inputs.DatasetParam[0]
	knowledgeInfos := datasetListInfoParam.Input.Value.Content.([]any)
	knowledgeInfoList := make([]*RetrieveKnowledgeInfo, 0, len(knowledgeInfos))
	for _, knowledgeInfo := range knowledgeInfos {
		retrieveKnowledgeInfo, err := buildRetrieveKnowledgeInfo(knowledgeInfo)
		if err != nil {
			return err
		}
		knowledgeInfoList = append(knowledgeInfoList, retrieveKnowledgeInfo)
	}
	c.KnowledgeInfos = knowledgeInfoList

	retrieveParams := &RetrieveParams{}

	var getDesignatedParamContent = func(name string) (any, bool) {
		for _, param := range inputs.DatasetParam {
			if param.Name == name {
				return param.Input.Value.Content, true
			}
		}
		return nil, false
	}

	if content, ok := getDesignatedParamContent("topK"); ok {
		topK, err := cast.ToInt64E(content)
		if err != nil {
			return err
		}
		retrieveParams.TopK = topK
	}

	if content, ok := getDesignatedParamContent("threshold"); ok {
		threshold, err := cast.ToFloat64E(content)
		if err != nil {
			return err
		}
		retrieveParams.Threshold = threshold
	}

	//vector（向量检索）、text（文本检索）、mix_rerank(混合 rerank模型)、mix_priority(混合权重)
	if content, ok := getDesignatedParamContent("matchType"); ok {
		matchType := cast.ToString(content)
		retrieveParams.MatchType = matchType
		if matchType == "mix_priority" {
			retrieveParams.PriorityMatch = 1
		}
	}

	if content, ok := getDesignatedParamContent("rerankModelId"); ok {
		retrieveParams.RerankModelId = cast.ToString(content)
	}

	if content, ok := getDesignatedParamContent("semanticsPriority"); ok {
		semanticsPriority, err := cast.ToFloat64E(content)
		if err != nil {
			return err
		}
		retrieveParams.SemanticsPriority = semanticsPriority
	}

	if content, ok := getDesignatedParamContent("keywordPriority"); ok {
		keywordPriority, err := cast.ToFloat64E(content)
		if err != nil {
			return err
		}
		retrieveParams.KeywordPriority = keywordPriority
	}

	if content, ok := getDesignatedParamContent("rerankKeywordPriority"); ok {
		rerankKeywordPriority, err := cast.ToFloat64E(content)
		if err != nil {
			return err
		}
		retrieveParams.RerankKeywordPriority = rerankKeywordPriority
	}

	if content, ok := getDesignatedParamContent("rerankKeywordPrioritySwitch"); ok {
		rerankKeywordPrioritySwitch, err := cast.ToBoolE(content)
		if err != nil {
			return err
		}
		retrieveParams.RerankKeywordPrioritySwitch = rerankKeywordPrioritySwitch
	}

	if content, ok := getDesignatedParamContent("rewrite"); ok {
		rewrite, err := cast.ToBoolE(content)
		if err != nil {
			return err
		}
		retrieveParams.Rewrite = rewrite
	}

	if content, ok := getDesignatedParamContent("useGraph"); ok {
		useGraph, err := cast.ToBoolE(content)
		if err != nil {
			return err
		}
		retrieveParams.UseGraph = useGraph
	}

	c.RetrieveParams = retrieveParams

	return nil
}

func (c *Config) setToolConfig(ctx context.Context, inputs *vo.Inputs) error {
	c.ToolParams = &ToolParams{
		PluginToolList: make([]*PluginToolInfo, 0),
	}
	for _, toolInfo := range inputs.AgentToolParams {
		var schemaStr string
		var auth *openapi3_util.Auth
		var err error
		schemaStr, auth, err = toolRequest(toolInfo.ToolID, toolInfo.ToolType, toolInfo.ApiKey)
		if err != nil {
			return fmt.Errorf("tool request err: %v", err)
		}

		var apiSchema *openapi3.T
		apiSchema, err = openapi3_util.LoadFromData(ctx, []byte(schemaStr))
		if err != nil {
			return fmt.Errorf("tool schema load err: %v", err)
		}

		c.ToolParams.PluginToolList = append(c.ToolParams.PluginToolList, &PluginToolInfo{
			APISchema: apiSchema,
			APIAuth:   auth,
		})
	}

	mcpInfoMap := make(map[string]*MCPToolInfo)
	for _, mcpInfo := range inputs.AgentMCPParams {
		info, exists := mcpInfoMap[mcpInfo.MCPID]
		if !exists {
			result, err := mcpRequest(mcpInfo.MCPID, mcpInfo.MCPType)
			if err != nil {
				return fmt.Errorf("mcp request failed: %w", err)
			}

			info = &MCPToolInfo{
				URL:          result.URL,
				Transport:    result.Transport,
				ToolNameList: make([]string, 0),
			}
			mcpInfoMap[mcpInfo.MCPID] = info
		}

		if mcpInfo.MCPToolName != "" {
			info.ToolNameList = append(info.ToolNameList, mcpInfo.MCPToolName)
		} else {
			return fmt.Errorf("mcp toolName empty, mcpID: %s", mcpInfo.MCPID)
		}
	}

	c.ToolParams.McpToolList = make([]*MCPToolInfo, 0, len(mcpInfoMap))
	for _, info := range mcpInfoMap {
		c.ToolParams.McpToolList = append(c.ToolParams.McpToolList, info)
	}

	if len(inputs.AgentWorkflowParams) > 0 {
		var workflowIDs []string
		for _, param := range inputs.AgentWorkflowParams {
			if param.WorkflowID != "" {
				workflowIDs = append(workflowIDs, param.WorkflowID)
			}
		}

		if len(workflowIDs) > 0 {
			result, err := getWorkflowSchemas(ctx, workflowIDs)
			if err != nil {
				return fmt.Errorf("failed to get workflow schemas: %w", err)
			}

			var schemas []json.RawMessage
			if err = json.Unmarshal(result, &schemas); err != nil {
				return fmt.Errorf("failed to unmarshal workflow schemas: %w", err)
			}

			for _, schemaByte := range schemas {
				if err := openapi3_util.ValidateSchema(ctx, schemaByte); err != nil {
					logs.CtxErrorf(ctx, "[AgentNode.Adapt] ValidateSchema failed for workflow schema: %v", err)
					return err
				}
				apiSchema, err := openapi3_util.LoadFromData(ctx, schemaByte)
				if err != nil {
					return fmt.Errorf("workflow schema load err: %v", err)
				}

				c.ToolParams.PluginToolList = append(c.ToolParams.PluginToolList, &PluginToolInfo{APISchema: apiSchema})
			}
		}
	}

	skillIdentities := make([]wanwu_util.SkillIdentity, 0, len(inputs.AgentSkillParams))
	for _, skillParam := range inputs.AgentSkillParams {
		skillIdentities = append(skillIdentities, wanwu_util.SkillIdentity{
			SkillID:   skillParam.SkillId,
			SkillType: skillParam.SkillType,
		})
	}
	if len(skillIdentities) > 0 {
		skillInfos, err := wanwu_util.FetchSkillToolInfoList(ctx, skillIdentities)
		if err != nil {
			return fmt.Errorf("skill request failed: %w", err)
		}
		c.ToolParams.SkillToolList = make([]*SkillToolInfo, 0, len(skillInfos))
		for _, info := range skillInfos {
			c.ToolParams.SkillToolList = append(c.ToolParams.SkillToolList, &SkillToolInfo{
				SkillId:    info.SkillId,
				SkillType:  SkillType(info.SkillType),
				Name:       info.Name,
				Desc:       info.Desc,
				Avatar:     info.Avatar,
				ObjectPath: info.ObjectPath,
			})
		}
	} else {
		c.ToolParams.SkillToolList = make([]*SkillToolInfo, 0)
	}

	return nil
}

type MCPInfo struct {
	SSEURL        string `json:"sseUrl"`
	StreamableURL string `json:"streamableUrl"`
	Transport     string `json:"transport"`
}

type MCPServerDetail struct {
	SSEURL        string `json:"sseUrl"`
	StreamableURL string `json:"streamableUrl"`
	Transport     string `json:"transport"`
}

type MCPRequestResult struct {
	URL       string
	Transport string
}

func mcpRequest(id, mcpType string) (*MCPRequestResult, error) {
	switch mcpType {
	case MCPTypeMCP:
		url, err := url.JoinPath(os.Getenv(WanWuMCPGetUrlEnv))
		if err != nil {
			return nil, err
		}
		var res response
		var ret MCPInfo
		resp, err := resty.New().SetTimeout(time.Minute).R().
			SetHeader("Content-Type", "application/json").
			SetHeader("Accept", "application/json").
			SetQueryParam("mcpId", id).
			SetResult(&res).Get(url)
		if err != nil {
			return nil, fmt.Errorf("request %v err: %v", url, err)
		}
		if resp.StatusCode() >= 300 {
			return nil, fmt.Errorf("request %v http status %v msg: %v", url, resp.StatusCode(), res.Msg)
		}
		marshal, err := sonic.Marshal(res.Data)
		if err != nil {
			return nil, fmt.Errorf("request %v marshal response body: %v", url, err)
		}
		if err = sonic.Unmarshal(marshal, &ret); err != nil {
			return nil, fmt.Errorf("request %v unmarshal response body: %v", url, err)
		}
		// 根据 transport 类型选择正确的 URL
		mcpReq, err := selectMCPUrl(ret.SSEURL, ret.StreamableURL, ret.Transport)
		if err != nil {
			return nil, fmt.Errorf("request %v err: %v", url, err)
		}
		return mcpReq, nil

	case MCPTypeMCPServer:
		url, err := url.JoinPath(os.Getenv(WanWuMCPServerGetUrlEnv))
		if err != nil {
			return nil, err
		}
		var res response
		var ret MCPServerDetail
		resp, err := resty.New().SetTimeout(time.Minute).R().
			SetHeader("Content-Type", "application/json").
			SetHeader("Accept", "application/json").
			SetQueryParam("mcpServerId", id).
			SetResult(&res).Get(url)
		if err != nil {
			return nil, fmt.Errorf("request %v err: %v", url, err)
		}
		if resp.StatusCode() >= 300 {
			return nil, fmt.Errorf("request %v http status %v msg: %v", url, resp.StatusCode(), res.Msg)
		}
		marshal, err := sonic.Marshal(res.Data)
		if err != nil {
			return nil, fmt.Errorf("request %v marshal response body: %v", url, err)
		}
		if err = sonic.Unmarshal(marshal, &ret); err != nil {
			return nil, fmt.Errorf("request %v unmarshal response body: %v", url, err)
		}
		// 根据 transport 类型选择正确的 URL
		mcpReq, err := selectMCPUrl(ret.SSEURL, ret.StreamableURL, ret.Transport)
		if err != nil {
			return nil, fmt.Errorf("request %v err: %v", url, err)
		}
		return mcpReq, nil
	}
	return nil, errors.New("unsupported mcp type")
}

// selectMCPUrl 根据 transport 类型选择正确的 URL
func selectMCPUrl(sseUrl, streamableUrl, transport string) (*MCPRequestResult, error) {
	switch transport {
	case MCPTransportStreamable:
		return &MCPRequestResult{URL: streamableUrl, Transport: MCPTransportStreamable}, nil
	case MCPTransportSSE:
		return &MCPRequestResult{URL: sseUrl, Transport: MCPTransportSSE}, nil
	default:
		return nil, fmt.Errorf("unsupported mcp transport %v", transport)
	}
}

func toolRequest(toolId, toolType, userApiKey string) (string, *openapi3_util.Auth, error) {
	switch toolType {
	case ToolTypeCustom:
		url, err := url.JoinPath(os.Getenv(WanWuCustomToolUrlEnv))
		if err != nil {
			return "", nil, err
		}
		var res response
		var ret customToolDetail
		resp, err := resty.New().SetTimeout(time.Minute).R().
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
	case ToolTypeBuiltIn:
		url, err := url.JoinPath(os.Getenv(WanWuSquareToolUrlEnv))
		if err != nil {
			return "", nil, err
		}
		var res response
		var ret toolSquareDetail
		resp, err := resty.New().SetTimeout(time.Minute).R().
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

func llmParamsToLLMParam(params vo.LLMParam) (*vo.LLMParams, error) {
	p := &vo.LLMParams{}
	for _, param := range params {
		switch param.Name {
		case "temperature":
			strVal := param.Input.Value.Content.(string)
			floatVal, err := strconv.ParseFloat(strVal, 64)
			if err != nil {
				return nil, err
			}
			p.Temperature = &floatVal
		case "maxTokens":
			strVal := param.Input.Value.Content.(string)
			intVal, err := strconv.Atoi(strVal)
			if err != nil {
				return nil, err
			}
			p.MaxTokens = intVal
		case "responseFormat":
			strVal := param.Input.Value.Content.(string)
			int64Val, err := strconv.ParseInt(strVal, 10, 64)
			if err != nil {
				return nil, err
			}
			p.ResponseFormat = vo.ResponseFormat(int64Val)
		case "modleName":
			strVal := param.Input.Value.Content.(string)
			p.ModelName = strVal
		case "modelType":
			strVal := param.Input.Value.Content.(string)
			int64Val, err := strconv.ParseInt(strVal, 10, 64)
			if err != nil {
				return nil, err
			}
			p.ModelType = int64Val
		case "prompt":
			strVal := param.Input.Value.Content.(string)
			p.Prompt = strVal
		case "enableChatHistory":
			boolVar := param.Input.Value.Content.(bool)
			p.EnableChatHistory = boolVar
		case "systemPrompt":
			strVal := param.Input.Value.Content.(string)
			p.SystemPrompt = strVal
		case "chatHistoryRound":
			strVal := param.Input.Value.Content.(string)
			int64Val, err := strconv.ParseInt(strVal, 10, 64)
			if err != nil {
				return nil, err
			}
			p.ChatHistoryRound = int64Val
		case "generationDiversity", "frequencyPenalty", "presencePenalty":
		// do nothing
		case "topP":
			strVal := param.Input.Value.Content.(string)
			floatVar, err := strconv.ParseFloat(strVal, 64)
			if err != nil {
				return nil, err
			}
			p.TopP = &floatVar
		case "thinkingType":
			strVal := param.Input.Value.Content.(string)
			p.ThinkingType = strVal
		default:
			logs.Warnf("encountered unknown param when converting LLM Params, name= %s, "+
				"value= %v", param.Name, param.Input.Value.Content)
		}
	}

	return p, nil
}

func (c *Config) Build(ctx context.Context, ns *schema2.NodeSchema, _ ...schema2.BuildOption) (any, error) {
	if c.LLMParams.ModelType == 0 {
		return nil, errors.New("model ID is required")
	}

	agent := &AgentNode{
		AgentBaseParams: c.AgentBaseParams,
		ModelParams: &ModelParams{
			ModelID:          strconv.FormatInt(c.LLMParams.ModelType, 10),
			Temperature:      float64PtrToFloat32Ptr(c.LLMParams.Temperature),
			TopP:             float64PtrToFloat32Ptr(c.LLMParams.TopP),
			FrequencyPenalty: float64ToFloat32Ptr(c.LLMParams.FrequencyPenalty),
			PresencePenalty:  float64ToFloat32Ptr(c.LLMParams.PresencePenalty),
			MaxTokens:        &c.LLMParams.MaxTokens,
		},
		KnowledgeParams: buildKnowledgeParams(ctx, c.KnowledgeInfos, c.RetrieveParams),
		ToolParams:      c.ToolParams,
		HttpClient: &http.Client{
			Transport: &http.Transport{
				ResponseHeaderTimeout: time.Minute,
			},
		},
	}
	switch c.LLMParams.ThinkingType {
	case "enabled":
		agent.ModelParams.EnableThinking = intPtr(1)
	case "disabled":
		agent.ModelParams.EnableThinking = intPtr(0)
	default:
	}

	return agent, nil
}

func intPtr(i int) *int {
	return &i
}

func float64PtrToFloat32Ptr(src *float64) *float32 {
	if src == nil {
		return nil
	}
	v := float32(*src)
	return &v
}

func float64ToFloat32Ptr(src float64) *float32 {
	v := float32(src)
	return &v
}

type AgentNode struct {
	AgentBaseParams *AgentBaseParams
	ModelParams     *ModelParams
	KnowledgeParams *KnowledgeParams
	ToolParams      *ToolParams
	HttpClient      *http.Client
}

type AgentChatRequest struct {
	Input           string           `json:"input"`
	UploadFile      []string         `json:"uploadFile"`
	Stream          bool             `json:"stream"`
	AgentBaseParams *AgentBaseParams `json:"agentBaseParams"`
	ModelParams     *ModelParams     `json:"modelParams"`
	KnowledgeParams *KnowledgeParams `json:"knowledgeParams"`
	ToolParams      *ToolParams      `json:"toolParams"`
}

type KnowledgeParams struct {
	UserId               string                 `json:"userId"`          // 用户id
	KnowledgeIdList      []string               `json:"knowledgeIdList"` // 知识库id列表
	Question             string                 `json:"question"`
	Threshold            float32                `json:"threshold"` // Score阈值
	TopK                 int32                  `json:"topK"`
	Stream               bool                   `json:"stream"`
	Chichat              bool                   `json:"chichat"` // 当知识库召回结果为空时是否使用默认话术（兜底），默认为true
	RerankModelId        string                 `json:"rerank_model_id"`
	CustomModelInfo      *CustomModelInfo       `json:"custom_model_info"`
	MaxHistory           int32                  `json:"max_history"`
	RewriteQuery         bool                   `json:"rewrite_query"`   // 是否query改写
	RerankMod            string                 `json:"rerank_mod"`      // rerank_model:重排序模式，weighted_score：权重搜索
	RetrieveMethod       string                 `json:"retrieve_method"` // hybrid_search:混合搜索， semantic_search:向量搜索， full_text_search：文本搜索
	Weight               *WeightParams          `json:"weights"`         // 权重搜索下的权重配置
	Temperature          float32                `json:"temperature,omitempty"`
	TopP                 float32                `json:"top_p,omitempty"`               // 多样性
	RepetitionPenalty    float32                `json:"repetition_penalty,omitempty"`  // 重复惩罚/频率惩罚
	ReturnMeta           bool                   `json:"return_meta,omitempty"`         // 是否返回元数据
	AutoCitation         bool                   `json:"auto_citation"`                 // 是否自动角标
	TermWeight           float32                `json:"term_weight_coefficient"`       // 关键词系数
	MetaFilter           bool                   `json:"metadata_filtering"`            // 元数据过滤开关
	MetaFilterConditions []*MetadataFilterParam `json:"metadata_filtering_conditions"` // 元数据过滤条件
	UseGraph             bool                   `json:"use_graph"`                     // 是否启动知识图谱查询
}

type CustomModelInfo struct {
	LlmModelID string `json:"llm_model_id"`
}

type ToolParams struct {
	PluginToolList []*PluginToolInfo `json:"pluginTool,omitempty"`
	McpToolList    []*MCPToolInfo    `json:"mcpToolList,omitempty"`
	SkillToolList  []*SkillToolInfo  `json:"skillToolList,omitempty"`
}

type AgentBaseParams struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Instruction string `json:"instruction"`
}

type ModelParams struct {
	ModelID          string   `json:"modelId"`
	Temperature      *float32 `json:"temperature,omitempty"`      //温度
	TopP             *float32 `json:"topP,omitempty"`             //topP
	FrequencyPenalty *float32 `json:"frequencyPenalty,omitempty"` //频率惩罚
	PresencePenalty  *float32 `json:"presence_penalty,omitempty"` //存在惩罚
	MaxTokens        *int     `json:"max_tokens,omitempty"`       //模型输出最大token数
	EnableThinking   *int     `json:"enable_thinking,omitempty"`  //是否启用思考
}

type PluginToolInfo struct {
	APISchema *openapi3.T         `json:"api_schema"`
	APIAuth   *openapi3_util.Auth `json:"api_auth,omitempty"`
}

type MCPToolInfo struct {
	URL          string   `json:"url"`
	Transport    string   `json:"transport"`
	ToolNameList []string `json:"toolNameList"` // MCP工具方法列表,会根据此方法名的列表进行mcp方法的过滤，如果此列为空，则标识不进行过滤
}

type SkillType string

type SkillToolInfo struct {
	SkillId    string    `json:"skillId"`
	SkillType  SkillType `json:"skillType"`
	Name       string    `json:"name"`
	Desc       string    `json:"desc"`
	Avatar     string    `json:"avatar"`
	ObjectPath string    `json:"objectPath"`
}

type EventData struct {
	Status    int    `json:"status"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	Profile   string `json:"profile"`
	TimeCost  string `json:"timeCost"`
	ParentID  string `json:"parentId"`
	Order     int    `json:"order"`
	EventType int    `json:"eventType"`
}

type SSEResponse struct {
	Code           int                    `json:"code"`
	Message        string                 `json:"message"`
	Response       string                 `json:"response"`
	Order          int                    `json:"order"`
	EventType      int                    `json:"eventType"`
	EventData      *EventData             `json:"eventData"`
	GenFileURLList []string               `json:"gen_file_url_list"`
	History        []any                  `json:"history"`
	Finish         int                    `json:"finish"`
	Usage          map[string]interface{} `json:"usage"`
	SearchList     []any                  `json:"search_list"`
	QAType         int                    `json:"qa_type"`
}

func (a *AgentNode) Invoke(ctx context.Context, input map[string]any) (map[string]any, error) {
	inputBytes, _ := json.Marshal(input)
	logs.CtxDebugf(ctx, "[AgentNode] Starting Invoke with input: %s", string(inputBytes))

	inputText, ok := input["query"].(string)
	if !ok || inputText == "" {
		return nil, errors.New("input field is required and must be a string")
	}

	req := &AgentChatRequest{
		Input:           inputText,
		Stream:          true,
		AgentBaseParams: a.AgentBaseParams,
		ModelParams:     a.ModelParams,
		KnowledgeParams: a.KnowledgeParams,
		ToolParams:      a.ToolParams,
	}

	uploadFileUrl, ok := input["file"].(string)
	if ok && uploadFileUrl != "" {
		req.UploadFile = append(req.UploadFile, uploadFileUrl)
	}

	reqBytes, _ := json.Marshal(req)
	logs.CtxDebugf(ctx, "[AgentNode] Built invoke request: %s", string(reqBytes))

	finalResult, err := a.callAgentService(ctx, req)
	if err != nil {
		logs.CtxErrorf(ctx, "[AgentNode] Call agent service failed: %v", err)
		return nil, err
	}

	// 构建兼容格式的输出
	var searchListStr string
	if len(finalResult.SearchList) > 0 {
		searchListBytes, _ := json.Marshal(finalResult.SearchList)
		searchListStr = string(searchListBytes)
	}

	logs.CtxDebugf(ctx, "[AgentNode] Invoke completed successfully with response: %s", finalResult.Response)

	result := make(map[string]any)
	result[AgentOutputKey] = map[string]any{
		"response":     finalResult.Response,
		"searchList":   searchListStr,
		"fullResponse": finalResultToMap(finalResult),
	}

	return result, nil
}

func (a *AgentNode) Stream(ctx context.Context, input map[string]any) (*schema.StreamReader[map[string]any], error) {
	inputBytes, _ := json.Marshal(input)
	logs.CtxDebugf(ctx, "[AgentNode] Starting Stream with input: %s", string(inputBytes))

	inputText, ok := input["query"].(string)
	if !ok || inputText == "" {
		return nil, errors.New("input field is required and must be a string")
	}

	req := &AgentChatRequest{
		Input:           inputText,
		Stream:          true,
		AgentBaseParams: a.AgentBaseParams,
		ModelParams:     a.ModelParams,
		KnowledgeParams: a.KnowledgeParams,
		ToolParams:      a.ToolParams,
	}

	uploadFileUrl, ok := input["file"].(string)
	if ok && uploadFileUrl != "" {
		req.UploadFile = append(req.UploadFile, uploadFileUrl)
	}

	reqBytes, _ := json.Marshal(req)
	logs.CtxDebugf(ctx, "[AgentNode] Built stream request: %s", string(reqBytes))

	return a.streamAgentService(ctx, req)
}

// SubConversation represents a single sub-conversation item in the response
type SubConversation struct {
	ID               string `json:"id"`
	Response         string `json:"response"`
	SearchList       any    `json:"searchList"`
	ParentID         string `json:"parentId"`
	Name             string `json:"name"`
	Profile          string `json:"profile"`
	TimeCost         string `json:"timeCost"`
	Status           int    `json:"status"`
	ConversationType string `json:"conversationType"`
	Order            int    `json:"order"`
}

type ResponseItem struct {
	Response string `json:"response"`
	Order    int    `json:"order"`
}

type FinalResult struct {
	ID                  string             `json:"id"`
	Response            string             `json:"response"`
	ResponseList        []ResponseItem     `json:"responseList"`
	SearchList          []any              `json:"searchList"`
	QAType              int                `json:"qa_type"`
	SubConversationList []*SubConversation `json:"subConversationList"`
}

// mapEventTypeToConversationType converts event type to conversation type string
func mapEventTypeToConversationType(eventType int) string {
	switch eventType {
	case 6:
		return "agentThink"
	case 3:
		return "agentTool"
	case 2:
		return "agentKnowledge"
	case 20:
		return "subText"
	default:
		return ""
	}
}

// buildFinalResult constructs the final result from collected maps
func buildFinalResult(responseMap map[int]string, eventMap map[int]*SubConversation, lastSearchList []any, lastQAType int) *FinalResult {
	var responseList []ResponseItem
	var lastResponse string

	orders := getSortedOrders(responseMap)
	for _, order := range orders {
		response := responseMap[order]
		responseList = append(responseList, ResponseItem{
			Response: response,
			Order:    order,
		})
		lastResponse = response // 保留最后一个
	}

	return &FinalResult{
		ID:                  "",
		Response:            lastResponse,
		ResponseList:        responseList,
		SearchList:          lastSearchList,
		QAType:              lastQAType,
		SubConversationList: getSubConversationList(eventMap),
	}
}

func getSortedOrders[T any](m map[int]T) []int {
	orders := make([]int, 0, len(m))
	for order := range m {
		orders = append(orders, order)
	}
	for i := 0; i < len(orders); i++ {
		for j := i + 1; j < len(orders); j++ {
			if orders[i] > orders[j] {
				orders[i], orders[j] = orders[j], orders[i]
			}
		}
	}
	return orders
}

// getSubConversationList extracts sub-conversation items from eventMap
func getSubConversationList(eventMap map[int]*SubConversation) []*SubConversation {
	items := make([]*SubConversation, 0, len(eventMap))
	for _, order := range getSortedOrders(eventMap) {
		items = append(items, eventMap[order])
	}
	return items
}

func (a *AgentNode) callAgentService(ctx context.Context, req *AgentChatRequest) (*FinalResult, error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	agentURL := os.Getenv(WanWuAgentAPIUrlEnv)
	logs.CtxDebugf(ctx, "[AgentNode] Sending request to %s: %s", agentURL, string(reqBody))

	httpReq, err := http.NewRequestWithContext(ctx, "POST", agentURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := a.HttpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	logs.CtxDebugf(ctx, "[AgentNode] Received response with status: %d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("agent service returned status %d: %s", resp.StatusCode, string(body))
	}

	scanner := bufio.NewScanner(resp.Body)
	responseMap := make(map[int]string)
	eventMap := make(map[int]*SubConversation)
	var lastSearchList []any
	var lastQAType int

	for scanner.Scan() {
		line := scanner.Text()
		logs.CtxDebugf(ctx, "[AgentNode] Received line: %s", line)

		if data, ok := strings.CutPrefix(line, "data:"); ok {
			var sseResp SSEResponse
			if err := json.Unmarshal([]byte(data), &sseResp); err != nil {
				logs.CtxWarnf(ctx, "[AgentNode] Failed to parse SSE response: %v, data: %s", err, data)
				continue
			}

			if sseResp.Code != 0 {
				return nil, fmt.Errorf("agent service error: code=%d, message=%s", sseResp.Code, sseResp.Message)
			}

			if sseResp.EventData == nil {
				if sseResp.Response != "" {
					responseMap[sseResp.Order] += sseResp.Response
				}
			} else {
				order := sseResp.EventData.Order
				if order == 0 {
					order = sseResp.Order
				}
				eventType := sseResp.EventData.EventType
				if eventType == 0 {
					eventType = sseResp.EventType
				}
				conversationType := mapEventTypeToConversationType(eventType)

				var searchListVal any
				if eventType == 2 && len(sseResp.SearchList) > 0 {
					searchListBytes, err := json.Marshal(sseResp.SearchList)
					if err == nil {
						searchListVal = string(searchListBytes)
					}
				}

				subConv, exists := eventMap[order]
				if !exists {
					subConv = &SubConversation{
						ID:               sseResp.EventData.ID,
						SearchList:       searchListVal,
						ParentID:         sseResp.EventData.ParentID,
						Name:             sseResp.EventData.Name,
						Profile:          sseResp.EventData.Profile,
						TimeCost:         sseResp.EventData.TimeCost,
						Status:           sseResp.EventData.Status,
						ConversationType: conversationType,
						Order:            order,
					}
					eventMap[order] = subConv
				}

				subConv.Response += sseResp.Response
				if searchListVal != nil {
					subConv.SearchList = searchListVal
				}
				if sseResp.EventData.Status != 0 {
					subConv.Status = sseResp.EventData.Status
				}
				if subConv.ID == "" {
					subConv.ID = sseResp.EventData.ID
				}
				if subConv.ParentID == "" {
					subConv.ParentID = sseResp.EventData.ParentID
				}
				if subConv.Name == "" {
					subConv.Name = sseResp.EventData.Name
				}
				if subConv.Profile == "" {
					subConv.Profile = sseResp.EventData.Profile
				}
				if subConv.TimeCost == "" {
					subConv.TimeCost = sseResp.EventData.TimeCost
				}
				if subConv.ConversationType == "" {
					subConv.ConversationType = conversationType
				}
			}

			if sseResp.Finish == 1 {
				lastSearchList = sseResp.SearchList
				lastQAType = sseResp.QAType
				logs.CtxDebugf(ctx, "[AgentNode] Stream finished")
				break
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("stream reading error: %w", err)
	}

	// 构建最终结果
	finalResult := buildFinalResult(responseMap, eventMap, lastSearchList, lastQAType)
	logs.CtxDebugf(ctx, "[AgentNode] Final result: response=%s, subConversations=%d", finalResult.Response, len(finalResult.SubConversationList))
	return finalResult, nil
}

func (a *AgentNode) streamAgentService(ctx context.Context, req *AgentChatRequest) (*schema.StreamReader[map[string]any], error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	agentURL := os.Getenv(WanWuAgentAPIUrlEnv)
	logs.CtxDebugf(ctx, "[AgentNode] Sending streaming request to %s: %s", agentURL, string(reqBody))

	httpReq, err := http.NewRequestWithContext(ctx, "POST", agentURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := a.HttpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send HTTP request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("agent service returned status %d: %s", resp.StatusCode, string(body))
	}

	logs.CtxDebugf(ctx, "[AgentNode] Started receiving streaming response")

	reader, writer := schema.Pipe[map[string]any](10)

	safego.Go(ctx, func() {
		defer resp.Body.Close()
		defer writer.Close()

		scanner := bufio.NewScanner(resp.Body)

		for scanner.Scan() {
			line := scanner.Text()
			logs.CtxDebugf(ctx, "[AgentNode] Received stream line: %s", line)

			if strings.HasPrefix(line, "data:") {
				data := strings.TrimPrefix(line, "data:")

				var sseResp SSEResponse
				if err := json.Unmarshal([]byte(data), &sseResp); err != nil {
					logs.CtxWarnf(ctx, "[AgentNode] Failed to parse SSE response: %v, data: %s", err, data)
					continue
				}

				if sseResp.Code != 0 {
					writer.Send(nil, fmt.Errorf("agent service error: code=%d, message=%s", sseResp.Code, sseResp.Message))
					return
				}

				if sseResp.Response != "" {
					writer.Send(map[string]any{
						AgentOutputKey: sseResp.Response,
					}, nil)
				}

				if sseResp.Finish == 1 {
					logs.CtxDebugf(ctx, "[AgentNode] Stream finished, sending completion marker")
					writer.Send(map[string]any{
						AgentOutputKey: nodes.KeyIsFinished,
					}, nil)
					return
				}
			}
		}

		if err := scanner.Err(); err != nil {
			logs.CtxErrorf(ctx, "[AgentNode] Scanner error: %v", err)
			writer.Send(nil, fmt.Errorf("stream reading error: %w", err))
		}
	})

	return reader, nil
}

type ListSchemaByWanwuReq struct {
	WorkflowIDs []string `json:"workflow_ids"`
}

func getWorkflowSchemas(ctx context.Context, workflowIDs []string) ([]byte, error) {
	reqBody, err := json.Marshal(ListSchemaByWanwuReq{WorkflowIDs: workflowIDs})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ListSchemaByWanwuReq: %w", err)
	}

	workflowListSchemaAPI := os.Getenv(WanWuWorkflowListSchemaUrlEnv)
	logs.CtxDebugf(ctx, "[AgentNode] Sending request to %s with workflowIDs: %v", workflowListSchemaAPI, workflowIDs)

	client := resty.New().SetTimeout(time.Minute)
	resp, err := client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(reqBody).
		Post(workflowListSchemaAPI)
	if err != nil {
		return nil, fmt.Errorf("request %v err: %v", workflowListSchemaAPI, err)
	}

	if resp.StatusCode() >= 300 {
		return nil, fmt.Errorf("request %v http status %v msg: %s", workflowListSchemaAPI, resp.StatusCode(), string(resp.Body()))
	}

	logs.CtxDebugf(ctx, "[AgentNode] Received workflow schemas response: %s", string(resp.Body()))
	return resp.Body(), nil
}

// buildKnowledgeParams 根据配置构建 KnowledgeParams。
// 注意：以下字段未在此方法中设置，保持零值或需在请求上下文中动态处理：
// - Question: 属于请求上下文特定的动态参数。
// - CustomModelInfo, MaxHistory: RetrieveParams 配置中未包含。
// - Temperature, TopP, RepetitionPenalty: RetrieveParams 配置中未包含。
// - ReturnMeta: RetrieveParams 配置中未包含。
func buildKnowledgeParams(ctx context.Context, knowledgeInfos []*RetrieveKnowledgeInfo, retrieveParams *RetrieveParams) *KnowledgeParams {
	if len(knowledgeInfos) == 0 || retrieveParams == nil {
		return nil
	}
	metaFilterConditions, _ := buildMetaDataFilterParams(knowledgeInfos)
	userId := ctxutil.MustGetUIDFromCtx(ctx)
	userIdStr := strconv.Itoa(int(userId))
	kp := &KnowledgeParams{
		UserId:               userIdStr,
		KnowledgeIdList:      buildKnowledgeIdList(knowledgeInfos),
		Threshold:            float32(retrieveParams.Threshold),
		TopK:                 int32(retrieveParams.TopK),
		RerankModelId:        retrieveParams.RerankModelId,
		RewriteQuery:         retrieveParams.Rewrite,
		UseGraph:             retrieveParams.UseGraph,
		MetaFilterConditions: metaFilterConditions,
		MetaFilter:           len(metaFilterConditions) > 0,
		Chichat:              false,
		AutoCitation:         true,
		Stream:               true,
	}

	switch retrieveParams.MatchType {
	case "vector":
		kp.RetrieveMethod = "semantic_search"
		kp.RerankMod = "rerank_model"
	case "text":
		kp.RetrieveMethod = "full_text_search"
	case "mix_priority":
		kp.RetrieveMethod = "hybrid_search"
		kp.RerankMod = "weighted_score"
		kp.Weight = &WeightParams{
			VectorWeight: retrieveParams.SemanticsPriority,
			TextWeight:   retrieveParams.KeywordPriority,
		}
	case "mix_rerank":
		kp.RetrieveMethod = "hybrid_search"
		kp.RerankMod = "rerank_model"
	}

	if retrieveParams.RerankKeywordPrioritySwitch {
		kp.TermWeight = float32(retrieveParams.RerankKeywordPriority)
	}

	return kp
}

// buildRetrieveKnowledgeInfo 经过一次序列化反序列化，效率一般
func buildRetrieveKnowledgeInfo(knowledgeInfo any) (*RetrieveKnowledgeInfo, error) {
	retrieveKnowledgeInfo := &RetrieveKnowledgeInfo{}
	k := cast.ToString(knowledgeInfo)
	if len(k) > 0 {
		retrieveKnowledgeInfo.Name = k
		return retrieveKnowledgeInfo, nil
	}
	marshal, err := json.Marshal(knowledgeInfo)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(marshal, retrieveKnowledgeInfo)
	if err != nil {
		return nil, err
	}
	metaDataFilterParams := retrieveKnowledgeInfo.MetaDataFilterParams
	if metaDataFilterParams != nil && metaDataFilterParams.FilterEnable &&
		len(metaDataFilterParams.MetaFilterParams) > 0 {
		for _, param := range metaDataFilterParams.MetaFilterParams {
			if param.Condition != "empty" && param.Value == "" {
				return nil, errors.New("metaDataFilterParams condition is not empty and value should not be empty")
			}
		}
	}
	return retrieveKnowledgeInfo, nil
}

// buildKnowledgeNameList 构造知识库名称
func buildKnowledgeIdList(knowledgeInfos []*RetrieveKnowledgeInfo) []string {
	if len(knowledgeInfos) == 0 {
		return make([]string, 0)
	}
	var nameList []string
	for _, info := range knowledgeInfos {
		nameList = append(nameList, info.KnowledgeId)
	}
	return nameList
}

// buildMetaDataFilterParams 构造元数据过滤参数
func buildMetaDataFilterParams(knowledgeInfos []*RetrieveKnowledgeInfo) ([]*MetadataFilterParam, error) {
	var ragMetaDataFilterParams []*MetadataFilterParam
	for _, k := range knowledgeInfos {
		if k.MetaDataFilterParams == nil || !k.MetaDataFilterParams.FilterEnable ||
			len(k.MetaDataFilterParams.MetaFilterParams) == 0 {
			continue
		}
		item, err := buildMetadataFilterItem(k.MetaDataFilterParams.MetaFilterParams)
		if err != nil {
			logs.Errorf("buildMetaDataFilterParams error %v", err)
			return nil, err
		}
		ragMetaDataFilterParams = append(ragMetaDataFilterParams, &MetadataFilterParam{
			FilterKnowledgeName: k.RagName,
			LogicalOperator:     k.MetaDataFilterParams.FilterLogicType,
			MetaList:            item,
		})
	}
	return ragMetaDataFilterParams, nil
}

// buildMetadataFilterItem 构造元数据过滤项
func buildMetadataFilterItem(metaFilterParams []*MetaFilterParams) ([]*MetadataFilterItem, error) {
	var ragMetaDataFilterItem []*MetadataFilterItem
	for _, k := range metaFilterParams {
		data, err := buildValueData(k.Type, k.Value, k.Condition)
		if err != nil {
			logs.Errorf("buildMetadataFilterItem error %v", err)
			return nil, err
		}
		ragMetaDataFilterItem = append(ragMetaDataFilterItem, &MetadataFilterItem{
			ComparisonOperator: k.Condition,
			MetaName:           k.Key,
			MetaType:           k.Type,
			Value:              data,
		})
	}
	return ragMetaDataFilterItem, nil
}

// buildValueData 进行值转换
func buildValueData(valueType string, value string, condition string) (interface{}, error) {
	if condition == "empty" {
		return nil, nil
	}
	switch valueType {
	case metaTypeNumber:
	case metaTypeTime:
		//valueResult, err := parseToTimestamp(value)
		//if err != nil || valueResult == 0 {
		//	return strconv.ParseInt(value, 10, 64)
		//}
		//return valueResult, nil
		return strconv.ParseInt(value, 10, 64)
	}
	return value, nil
}

// finalResultToMap 将 FinalResult 转换为 map[string]any 格式
// 这样可以被框架的 convertToObject 正确处理
func finalResultToMap(result *FinalResult) map[string]any {
	if result == nil {
		return nil
	}

	// 将 ResponseItem 列表转换为 []any
	var responseListAny []any
	for _, item := range result.ResponseList {
		responseListAny = append(responseListAny, map[string]any{
			"response": item.Response,
			"order":    item.Order,
		})
	}

	// 将 SubConversation 列表转换为 []any
	var subConvListAny []any
	for _, subConv := range result.SubConversationList {
		subConvListAny = append(subConvListAny, map[string]any{
			"id":               subConv.ID,
			"response":         subConv.Response,
			"searchList":       subConv.SearchList,
			"parentId":         subConv.ParentID,
			"name":             subConv.Name,
			"profile":          subConv.Profile,
			"timeCost":         subConv.TimeCost,
			"status":           subConv.Status,
			"conversationType": subConv.ConversationType,
			"order":            subConv.Order,
		})
	}

	return map[string]any{
		"id":                  result.ID,
		"response":            result.Response,
		"responseList":        responseListAny,
		"searchList":          result.SearchList,
		"qa_type":             result.QAType,
		"subConversationList": subConvListAny,
	}
}
