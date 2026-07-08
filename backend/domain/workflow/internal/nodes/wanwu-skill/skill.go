package wanwu_skill

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/schema"
	"github.com/coze-dev/coze-studio/backend/domain/workflow"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/entity"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/entity/vo"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/internal/canvas/convert"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/internal/execute"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/internal/nodes"
	wanwu_util "github.com/coze-dev/coze-studio/backend/domain/workflow/internal/nodes/wanwu-util"
	schema2 "github.com/coze-dev/coze-studio/backend/domain/workflow/internal/schema"
	http_client "github.com/coze-dev/coze-studio/backend/pkg/http-client"
	"github.com/coze-dev/coze-studio/backend/pkg/logs"
	"github.com/coze-dev/coze-studio/backend/pkg/safego"
)

const (
	WanWuAgentAPIUrlEnv = "WANWU_AGENT_API_URL"
	AgentOutputKey      = "output"
)

type Config struct {
	AgentBaseParams  *AgentBaseParams
	LLMParams        *vo.LLMParams
	SkillIdentities  []wanwu_util.SkillIdentity
}

type AgentBaseParams struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Instruction string `json:"instruction"`
}

type ModelParams struct {
	ModelID          string   `json:"modelId"`
	Temperature      *float32 `json:"temperature,omitempty"`
	TopP             *float32 `json:"topP,omitempty"`
	FrequencyPenalty *float32 `json:"frequencyPenalty,omitempty"`
	PresencePenalty  *float32 `json:"presence_penalty,omitempty"`
	MaxTokens        *int     `json:"max_tokens,omitempty"`
	EnableThinking   *int     `json:"enable_thinking,omitempty"`
}

type SkillType string

type SkillToolInfo struct {
	SkillId    string                     `json:"skillId"`
	SkillType  SkillType                  `json:"skillType"`
	Name       string                     `json:"name"`
	Desc       string                     `json:"desc"`
	Avatar     string                     `json:"avatar"`
	ObjectPath string                     `json:"objectPath"`
	Variables  []wanwu_util.SkillVariable `json:"variables,omitempty"`
}

// redactSkillChatRequest 返回 req 的浅拷贝副本，其中 SkillToolList 内的 VariableValue 被替换为 "<redacted>"
// 用于请求体日志打印，避免敏感变量值进入日志（安全约束：VariableValue 不得进入 LLM 上下文 / 日志）
func redactSkillChatRequest(req *SkillChatRequest) *SkillChatRequest {
	if req == nil {
		return nil
	}
	cloned := *req
	if req.ToolParams == nil {
		return &cloned
	}
	tp := *req.ToolParams
	tp.SkillToolList = redactSkillToolList(tp.SkillToolList)
	cloned.ToolParams = &tp
	return &cloned
}

// redactSkillToolList 返回 list 的浅拷贝副本，其中 VariableValue 被替换为 "<redacted>"
func redactSkillToolList(list []*SkillToolInfo) []*SkillToolInfo {
	if len(list) == 0 {
		return list
	}
	redactedList := make([]*SkillToolInfo, len(list))
	for i, s := range list {
		if s == nil {
			redactedList[i] = nil
			continue
		}
		rc := *s
		if len(rc.Variables) > 0 {
			redactedVars := make([]wanwu_util.SkillVariable, len(rc.Variables))
			for j, v := range rc.Variables {
				redactedVars[j] = v
				redactedVars[j].VariableValue = "<redacted>"
			}
			rc.Variables = redactedVars
		}
		redactedList[i] = &rc
	}
	return redactedList
}

type ToolParams struct {
	SkillToolList []*SkillToolInfo `json:"skillToolList,omitempty"`
}

type WanWuSkill struct {
	AgentBaseParams *AgentBaseParams
	ModelParams     *ModelParams
	SkillIdentities []wanwu_util.SkillIdentity
	HttpClient      *http.Client
}

type SkillChatRequest struct {
	Input           string           `json:"input"`
	UploadFile      []string         `json:"uploadFile"`
	Stream          bool             `json:"stream"`
	AgentBaseParams *AgentBaseParams `json:"agentBaseParams"`
	ModelParams     *ModelParams     `json:"modelParams"`
	ToolParams      *ToolParams      `json:"toolParams"`
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

func (c *Config) Adapt(ctx context.Context, n *vo.Node, _ ...nodes.AdaptOption) (*schema2.NodeSchema, error) {
	ns := &schema2.NodeSchema{
		Key:     vo.NodeKey(n.ID),
		Type:    entity.NodeTypeWanWuSkill,
		Name:    n.Data.Meta.Title,
		Configs: c,
	}

	inputs := n.Data.Inputs
	meta := n.Data.Meta

	if err := c.setLLMConfig(ctx, inputs, meta); err != nil {
		return nil, err
	}

	if err := c.setSkillConfig(ctx, inputs); err != nil {
		return nil, err
	}

	if err := convert.SetInputsForNodeSchema(n, ns); err != nil {
		return nil, err
	}

	if err := convert.SetOutputTypesForNodeSchema(n, ns); err != nil {
		return nil, err
	}

	if redactedCB, rErr := json.Marshal(c); rErr == nil {
		logs.CtxDebugf(ctx, "[SkillNode] skill config: %s", string(redactedCB))
	} else {
		logs.CtxWarnf(ctx, "[SkillNode] marshal skill config failed: %v", rErr)
	}
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

	logs.CtxDebugf(ctx, "[SkillNode.Adapt] Extracted config - ModelID: %s, ModelName: %s",
		c.LLMParams.ModelType, c.LLMParams.ModelName)

	c.AgentBaseParams = &AgentBaseParams{
		Name:        meta.Title,
		Description: meta.Description,
		Instruction: convertedLLMParam.SystemPrompt,
	}

	return nil
}

func (c *Config) setSkillConfig(ctx context.Context, inputs *vo.Inputs) error {
	skillIdentities := make([]wanwu_util.SkillIdentity, 0, len(inputs.AgentSkillParams))
	for _, skillParam := range inputs.AgentSkillParams {
		skillIdentities = append(skillIdentities, wanwu_util.SkillIdentity{
			SkillID:   skillParam.SkillId,
			SkillType: skillParam.SkillType,
		})
	}
	c.SkillIdentities = skillIdentities
	return nil
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

	skill := &WanWuSkill{
		AgentBaseParams: c.AgentBaseParams,
		ModelParams: &ModelParams{
			ModelID:          strconv.FormatInt(c.LLMParams.ModelType, 10),
			Temperature:      float64PtrToFloat32Ptr(c.LLMParams.Temperature),
			TopP:             float64PtrToFloat32Ptr(c.LLMParams.TopP),
			FrequencyPenalty: float64ToFloat32Ptr(c.LLMParams.FrequencyPenalty),
			PresencePenalty:  float64ToFloat32Ptr(c.LLMParams.PresencePenalty),
			MaxTokens:        &c.LLMParams.MaxTokens,
		},
		SkillIdentities: c.SkillIdentities,
		HttpClient: &http.Client{
			Transport: http_client.GetClient().Client.Transport,
		},
	}
	switch c.LLMParams.ThinkingType {
	case "enabled":
		skill.ModelParams.EnableThinking = intPtr(1)
	case "disabled":
		skill.ModelParams.EnableThinking = intPtr(0)
	default:
	}

	return skill, nil
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

func (s *WanWuSkill) Invoke(ctx context.Context, input map[string]any) (map[string]any, error) {
	inputBytes, _ := json.Marshal(input)
	logs.CtxDebugf(ctx, "[SkillNode] Starting Invoke with input: %s", string(inputBytes))

	inputText, ok := input["query"].(string)
	if !ok || inputText == "" {
		return nil, errors.New("input field is required and must be a string")
	}

	skillToolList, err := s.fetchSkillToolList(ctx)
	if err != nil {
		return nil, err
	}

	req := &SkillChatRequest{
		Input:           inputText,
		Stream:          true,
		AgentBaseParams: s.AgentBaseParams,
		ModelParams:     s.ModelParams,
		ToolParams: &ToolParams{
			SkillToolList: skillToolList,
		},
	}

	uploadFileUrl, ok := input["file"].(string)
	if ok && uploadFileUrl != "" {
		req.UploadFile = append(req.UploadFile, uploadFileUrl)
	}

	if redactedBytes, rErr := json.Marshal(redactSkillChatRequest(req)); rErr == nil {
		logs.CtxDebugf(ctx, "[SkillNode] Built invoke request: %s", string(redactedBytes))
	} else {
		logs.CtxWarnf(ctx, "[SkillNode] marshal redacted invoke request failed: %v", rErr)
	}

	finalResult, err := s.callSkillService(ctx, req)
	if err != nil {
		logs.CtxErrorf(ctx, "[SkillNode] Call skill service failed: %v", err)
		return nil, err
	}

	logs.CtxDebugf(ctx, "[SkillNode] Invoke completed successfully with response: %s", finalResult.Response)

	result := make(map[string]any)
	result[AgentOutputKey] = map[string]any{
		"response":     finalResult.Response,
		"fullResponse": finalResultToMap(finalResult),
	}

	return result, nil
}

func (s *WanWuSkill) Stream(ctx context.Context, input map[string]any) (*schema.StreamReader[map[string]any], error) {
	inputBytes, _ := json.Marshal(input)
	logs.CtxDebugf(ctx, "[SkillNode] Starting Stream with input: %s", string(inputBytes))

	inputText, ok := input["query"].(string)
	if !ok || inputText == "" {
		return nil, errors.New("input field is required and must be a string")
	}

	skillToolList, err := s.fetchSkillToolList(ctx)
	if err != nil {
		return nil, err
	}

	req := &SkillChatRequest{
		Input:           inputText,
		Stream:          true,
		AgentBaseParams: s.AgentBaseParams,
		ModelParams:     s.ModelParams,
		ToolParams: &ToolParams{
			SkillToolList: skillToolList,
		},
	}

	uploadFileUrl, ok := input["file"].(string)
	if ok && uploadFileUrl != "" {
		req.UploadFile = append(req.UploadFile, uploadFileUrl)
	}

	if redactedBytes, rErr := json.Marshal(redactSkillChatRequest(req)); rErr == nil {
		logs.CtxDebugf(ctx, "[SkillNode] Built stream request: %s", string(redactedBytes))
	} else {
		logs.CtxWarnf(ctx, "[SkillNode] marshal redacted stream request failed: %v", rErr)
	}

	return s.streamSkillService(ctx, req)
}

func (s *WanWuSkill) callSkillService(ctx context.Context, req *SkillChatRequest) (*FinalResult, error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	agentURL := os.Getenv(WanWuAgentAPIUrlEnv)
	if redactedBody, rErr := json.Marshal(redactSkillChatRequest(req)); rErr == nil {
		logs.CtxDebugf(ctx, "[SkillNode] Sending request to %s: %s", agentURL, string(redactedBody))
	} else {
		logs.CtxWarnf(ctx, "[SkillNode] marshal redacted request failed: %v", rErr)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", agentURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := s.HttpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	logs.CtxDebugf(ctx, "[SkillNode] Received response with status: %d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("skill service returned status %d: %s", resp.StatusCode, string(body))
	}

	scanner := wanwu_util.NewScanner(resp.Body)
	responseMap := make(map[int]string)
	eventMap := make(map[int]*SubConversation)
	var lastSearchList []any
	var lastQAType int

	for scanner.Scan() {
		line := scanner.Text()
		logs.CtxDebugf(ctx, "[SkillNode] Received line: %s", line)

		if data, ok := strings.CutPrefix(line, "data:"); ok {
			var sseResp SSEResponse
			if err := json.Unmarshal([]byte(data), &sseResp); err != nil {
				logs.CtxWarnf(ctx, "[SkillNode] Failed to parse SSE response: %v, data: %s", err, data)
				continue
			}

			if sseResp.Code != 0 {
				return nil, fmt.Errorf("skill service error: code=%d, message=%s", sseResp.Code, sseResp.Message)
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
				logs.CtxDebugf(ctx, "[SkillNode] Stream finished")
				break
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("stream reading error: %w", err)
	}

	finalResult := buildFinalResult(responseMap, eventMap, lastSearchList, lastQAType)
	logs.CtxDebugf(ctx, "[SkillNode] Final result: response=%s, subConversations=%d", finalResult.Response, len(finalResult.SubConversationList))
	return finalResult, nil
}

func (s *WanWuSkill) streamSkillService(ctx context.Context, req *SkillChatRequest) (*schema.StreamReader[map[string]any], error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	agentURL := os.Getenv(WanWuAgentAPIUrlEnv)
	if redactedBody, rErr := json.Marshal(redactSkillChatRequest(req)); rErr == nil {
		logs.CtxDebugf(ctx, "[SkillNode] Sending streaming request to %s: %s", agentURL, string(redactedBody))
	} else {
		logs.CtxWarnf(ctx, "[SkillNode] marshal redacted streaming request failed: %v", rErr)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", agentURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := s.HttpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send HTTP request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("skill service returned status %d: %s", resp.StatusCode, string(body))
	}

	logs.CtxDebugf(ctx, "[SkillNode] Started receiving streaming response")

	reader, writer := schema.Pipe[map[string]any](10)

	safego.Go(ctx, func() {
		defer resp.Body.Close()
		defer writer.Close()

		scanner := wanwu_util.NewScanner(resp.Body)

		for scanner.Scan() {
			line := scanner.Text()
			logs.CtxDebugf(ctx, "[SkillNode] Received stream line: %s", line)

			if strings.HasPrefix(line, "data:") {
				data := strings.TrimPrefix(line, "data:")

				var sseResp SSEResponse
				if err := json.Unmarshal([]byte(data), &sseResp); err != nil {
					logs.CtxWarnf(ctx, "[SkillNode] Failed to parse SSE response: %v, data: %s", err, data)
					continue
				}

				if sseResp.Code != 0 {
					writer.Send(nil, fmt.Errorf("skill service error: code=%d, message=%s", sseResp.Code, sseResp.Message))
					return
				}

				if sseResp.Response != "" {
					writer.Send(map[string]any{
						AgentOutputKey: sseResp.Response,
					}, nil)
				}

				if sseResp.Finish == 1 {
					logs.CtxDebugf(ctx, "[SkillNode] Stream finished, sending completion marker")
					writer.Send(map[string]any{
						AgentOutputKey: nodes.KeyIsFinished,
					}, nil)
					return
				}
			}
		}

		if err := scanner.Err(); err != nil {
			logs.CtxErrorf(ctx, "[SkillNode] Scanner error: %v", err)
			writer.Send(nil, fmt.Errorf("stream reading error: %w", err))
		}
	})

	return reader, nil
}

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
		lastResponse = response
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

func getSubConversationList(eventMap map[int]*SubConversation) []*SubConversation {
	items := make([]*SubConversation, 0, len(eventMap))
	for _, order := range getSortedOrders(eventMap) {
		items = append(items, eventMap[order])
	}
	return items
}

func finalResultToMap(result *FinalResult) map[string]any {
	if result == nil {
		return nil
	}

	var responseListAny []any
	for _, item := range result.ResponseList {
		responseListAny = append(responseListAny, map[string]any{
			"response": item.Response,
			"order":    item.Order,
		})
	}

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

// fetchSkillToolList 在运行时（Invoke/Stream）拉取 skill 详情，含 per-user variables
// 必须在运行时调用：Adapt/Build 阶段 ctx 里没有 execute.Context，拿不到 workflow owner
func (s *WanWuSkill) fetchSkillToolList(ctx context.Context) ([]*SkillToolInfo, error) {
	if len(s.SkillIdentities) == 0 {
		return make([]*SkillToolInfo, 0), nil
	}
	ownerUserID, ownerOrgID, err := fetchWorkflowOwnerAsString(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch workflow owner for skill failed: %w", err)
	}
	skillInfos, err := wanwu_util.FetchSkillToolInfoList(ctx, s.SkillIdentities, ownerUserID, ownerOrgID)
	if err != nil {
		return nil, fmt.Errorf("skill request failed: %w", err)
	}
	list := make([]*SkillToolInfo, 0, len(skillInfos))
	for _, info := range skillInfos {
		list = append(list, &SkillToolInfo{
			SkillId:    info.SkillId,
			SkillType:  SkillType(info.SkillType),
			Name:       info.Name,
			Desc:       info.Desc,
			Avatar:     info.Avatar,
			ObjectPath: info.ObjectPath,
			Variables:  info.Variables,
		})
	}
	return list, nil
}

// fetchWorkflowOwnerAsString 返回工作流 owner 的 userId 和 orgId（string）
// 数据源：wanwu-workflow 本地 workflow_meta 表的 creator_id / space_id
// 这两列在 CreateWorkflowByWanwu 时写入，等于 wanwu 主仓库的 userId/orgId，不依赖发布状态。
// 用于 BFF skill callback 请求体里的 userId/orgId 字段——设计文档要求传工作流 owner 身份拉 per-user 变量。
func fetchWorkflowOwnerAsString(ctx context.Context) (userID, orgID string, err error) {
	exeCtx := execute.GetExeCtx(ctx)
	if exeCtx == nil || exeCtx.RootCtx.RootWorkflowBasic == nil {
		err = errors.New("workflow basic not found in exe ctx")
		logs.CtxWarnf(ctx, "[SkillNode.fetchWorkflowOwner] failed: %v", err)
		return "", "", err
	}
	wfID := exeCtx.RootCtx.RootWorkflowBasic.ID
	if wfID == 0 {
		err = errors.New("workflow id is empty")
		logs.CtxWarnf(ctx, "[SkillNode.fetchWorkflowOwner] failed: %v", err)
		return "", "", err
	}
	meta, err := workflow.GetRepository().GetMeta(ctx, wfID)
	if err != nil {
		err = fmt.Errorf("get workflow meta failed: %w", err)
		logs.CtxErrorf(ctx, "[SkillNode.fetchWorkflowOwner] workflowID=%d failed: %v", wfID, err)
		return "", "", err
	}
	if meta == nil {
		err = errors.New("workflow meta is nil")
		logs.CtxWarnf(ctx, "[SkillNode.fetchWorkflowOwner] workflowID=%d failed: %v", wfID, err)
		return "", "", err
	}
	if meta.CreatorID == 0 {
		err = errors.New("workflow creator id is empty")
		logs.CtxWarnf(ctx, "[SkillNode.fetchWorkflowOwner] workflowID=%d failed: %v", wfID, err)
		return "", "", err
	}
	if meta.SpaceID == 0 {
		err = errors.New("workflow space id is empty")
		logs.CtxWarnf(ctx, "[SkillNode.fetchWorkflowOwner] workflowID=%d failed: %v", wfID, err)
		return "", "", err
	}
	logs.CtxInfof(ctx, "[SkillNode.fetchWorkflowOwner] workflowID=%d owner userID=%d orgID=%d",
		wfID, meta.CreatorID, meta.SpaceID)
	return strconv.FormatInt(meta.CreatorID, 10), strconv.FormatInt(meta.SpaceID, 10), nil
}
