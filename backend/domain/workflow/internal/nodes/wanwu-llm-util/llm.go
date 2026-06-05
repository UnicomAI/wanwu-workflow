package wanwu_llm_util

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/libs/acl/openai"
	"github.com/coze-dev/coze-studio/backend/api/model/admin/config"
	"github.com/coze-dev/coze-studio/backend/api/model/app/developer_api"
	"github.com/coze-dev/coze-studio/backend/bizpkg/config/modelmgr"
	"github.com/coze-dev/coze-studio/backend/bizpkg/llm/modelbuilder"
	chatmodel "github.com/coze-dev/coze-studio/backend/bizpkg/llm/wanwu-chatmodel"
	chatmodelImpl "github.com/coze-dev/coze-studio/backend/bizpkg/llm/wanwu-chatmodel/impl/chatmodel"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/entity/vo"
	http_client "github.com/coze-dev/coze-studio/backend/pkg/http-client"
	"github.com/coze-dev/coze-studio/backend/pkg/lang/ptr"
)

func CreateChatModel(ctx context.Context, llmParams *vo.LLMParams) (modelbuilder.ToolCallingChatModel, *modelmgr.Model, error) {
	if llmParams == nil {
		return nil, nil, errors.New("empty llmParams")
	}
	var topP *float32
	if llmParams.TopP != nil {
		topPF32 := float32(*llmParams.TopP)
		topP = &topPF32
	}
	var temperature *float32
	if llmParams.Temperature != nil {
		temperatureF32 := float32(*llmParams.Temperature)
		temperature = &temperatureF32
	}
	var maxTokens *int
	if llmParams.MaxTokens != 0 {
		maxTokens = &llmParams.MaxTokens
	}
	var responseFormatType openai.ChatCompletionResponseFormatType
	switch llmParams.ResponseFormat {
	case vo.ResponseFormatText:
		responseFormatType = openai.ChatCompletionResponseFormatTypeText
	case vo.ResponseFormatMarkdown:
		responseFormatType = openai.ChatCompletionResponseFormatTypeJSONObject
	case vo.ResponseFormatJSON:
		responseFormatType = openai.ChatCompletionResponseFormatTypeJSONObject
	default:
		responseFormatType = openai.ChatCompletionResponseFormatTypeText
	}
	baseUrl, err := url.JoinPath(os.Getenv("WANWU_CALLBACK_LLM_BASE_URL"), strconv.Itoa(int(llmParams.ModelType)))
	if err != nil {
		return nil, nil, err
	}

	// 根据 ThinkingType 设置 EnableThinking
	// 前端传递的字符串: "disabled", "enabled"
	var enableThinking *bool
	if llmParams.ThinkingType != "" {
		switch llmParams.ThinkingType {
		case "enabled":
			enableThinking = ptr.Of(true)
		case "disabled":
			enableThinking = ptr.Of(false)
		default:
			enableThinking = nil
		}
	}

	// chatmodel
	m, err := chatmodelImpl.NewDefaultFactory().CreateChatModel(ctx, chatmodel.ProtocolOpenAI, &chatmodel.Config{
		BaseURL:        baseUrl,
		Model:          llmParams.ModelName,
		TopP:           topP,
		Temperature:    temperature,
		MaxTokens:      maxTokens,
		EnableThinking: enableThinking,
		OpenAI:         &chatmodel.OpenAIConfig{ResponseFormat: &openai.ChatCompletionResponseFormat{Type: responseFormatType}},
	})
	if err != nil {
		return nil, nil, err
	}
	// modelmgr.Model capability
	resp, err := http_client.GetRestyClientWithTimeout(time.Minute).R().SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetDoNotParseResponse(true).Get(baseUrl)
	if err != nil {
		return nil, nil, fmt.Errorf("request %v err: %v", baseUrl, err)
	}
	b, err := io.ReadAll(resp.RawResponse.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("request %v read response body: %v", baseUrl, err)
	}
	if resp.StatusCode() >= 300 {
		return nil, nil, fmt.Errorf("request %v http status %v msg: %v", baseUrl, resp.StatusCode(), string(b))
	}
	var ret *modelResp
	if err = sonic.Unmarshal(b, &ret); err != nil {
		return nil, nil, fmt.Errorf("request %v read response body: %v", baseUrl, err)
	}
	functionCall := ret.Data.Config.FunctionCall == "toolCall"
	imageUnerstanding := ret.Data.Config.VisionSupport == "support"
	modelInfo := &modelmgr.Model{
		Model: &config.Model{
			Capability: &developer_api.ModelAbility{
				FunctionCall:       &functionCall,
				ImageUnderstanding: &imageUnerstanding,
				SupportMultiModal:  &imageUnerstanding,
			},
		},
	}
	return m, modelInfo, nil
}

type modelResp struct {
	Code int       `json:"code"`
	Data modelInfo `json:"data"`
	Msg  string    `json:"msg"`
}

type modelInfo struct {
	Config modelConfig `json:"config"`
}

type modelConfig struct {
	FunctionCall  string `json:"functionCalling"`
	VisionSupport string `json:"visionSupport"`
}
