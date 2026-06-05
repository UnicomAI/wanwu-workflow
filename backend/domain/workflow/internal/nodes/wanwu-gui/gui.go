package wanwu_gui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"time"

	"github.com/bytedance/sonic"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/entity"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/entity/vo"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/internal/canvas/convert"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/internal/nodes"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/internal/schema"
	http_client "github.com/coze-dev/coze-studio/backend/pkg/http-client"
)

type Config struct {
	ModelID string
}

func (c *Config) Adapt(_ context.Context, n *vo.Node, _ ...nodes.AdaptOption) (*schema.NodeSchema, error) {
	ns := &schema.NodeSchema{
		Key:     vo.NodeKey(n.ID),
		Type:    entity.NodeTypeWanWuGUI,
		Name:    n.Data.Meta.Title,
		Configs: c,
	}

	if n.Data.Inputs.WanwuGUIParam == nil {
		return nil, fmt.Errorf("GUI node's guiParams is nil")
	}
	c.ModelID = n.Data.Inputs.WanwuGUIParam.ModelID

	if err := convert.SetInputsForNodeSchema(n, ns); err != nil {
		return nil, err
	}

	if err := convert.SetOutputTypesForNodeSchema(n, ns); err != nil {
		return nil, err
	}

	return ns, nil
}

func (c *Config) Build(_ context.Context, _ *schema.NodeSchema, _ ...schema.BuildOption) (any, error) {
	if c.ModelID == "" {
		return nil, errors.New("config gui model id is required")
	}
	return &GUI{
		ModelID: c.ModelID,
	}, nil
}

type GUI struct {
	ModelID string
}

func (gui *GUI) Invoke(ctx context.Context, input map[string]any) (map[string]any, error) {
	if input == nil {
		return nil, fmt.Errorf("input data for GUI cannot be nil")
	}
	if _, ok := input["platform"]; !ok {
		return nil, errors.New("input platform field required")
	}
	if _, ok := input["current_screenshot"]; !ok {
		return nil, errors.New("input current_screenshot field required")
	}
	if _, ok := input["current_screenshot_width"]; !ok {
		return nil, errors.New("input current_screenshot_width field required")
	}
	if _, ok := input["current_screenshot_height"]; !ok {
		return nil, errors.New("input current_screenshot_height field required")
	}
	if _, ok := input["task"]; !ok {
		return nil, errors.New("input task field required")
	}
	inputBytes, err := sonic.Marshal(input)
	if err != nil {
		return nil, err
	}
	req := &guiReq{}
	if err = sonic.Unmarshal(inputBytes, req); err != nil {
		return nil, err
	}
	req.Algo = "gui_agent_v1"
	if req.History == nil {
		req.History = make([]string, 0)
	}
	return guiRequest(ctx, gui.ModelID, req)
}

type guiReq struct {
	Algo                    string   `json:"algo"`
	Platform                string   `json:"platform"`
	CurrentScreenShot       string   `json:"current_screenshot"`
	CurrentScreenShotWidth  int32    `json:"current_screenshot_width"`
	CurrentScreenShotHeight int32    `json:"current_screenshot_height"`
	Task                    string   `json:"task"`
	History                 []string `json:"history"`
}

func guiRequest(ctx context.Context, modelID string, req *guiReq) (map[string]any, error) {
	url, err := url.JoinPath(os.Getenv("WANWU_CALLBACK_LLM_BASE_URL"), modelID, "gui")
	if err != nil {
		return nil, err
	}
	resp, err := http_client.GetRestyClientWithTimeout(time.Minute).R().SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBody(req).
		SetDoNotParseResponse(true).Post(url)
	if err != nil {
		return nil, fmt.Errorf("request %v err: %v", url, err)
	}
	b, err := io.ReadAll(resp.RawResponse.Body)
	if err != nil {
		return nil, fmt.Errorf("request %v read response body: %v", url, err)
	}
	if resp.StatusCode() >= 300 {
		return nil, fmt.Errorf("request %v http status %v msg: %v", url, resp.StatusCode(), string(b))
	}
	var ret map[string]any
	if err = sonic.Unmarshal(b, &ret); err != nil {
		return nil, fmt.Errorf("request %v unmarshal response body: %v", url, err)
	}
	return ret, nil
}
