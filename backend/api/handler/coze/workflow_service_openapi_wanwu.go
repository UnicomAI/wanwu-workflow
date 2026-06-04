package coze

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/coze-dev/coze-studio/backend/api/model/workflow"
	appworkflow "github.com/coze-dev/coze-studio/backend/application/workflow"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/entity"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/entity/vo"
	"github.com/coze-dev/coze-studio/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-studio/backend/pkg/logs"
	"github.com/coze-dev/coze-studio/backend/pkg/sonic"
)

// ListWorkFlowOpenAPIV3SchemaByWanwu 获取workflow list openapi v3 schema
// @router /v1/workflow/list_schema_by_wanwu [POST]
func ListWorkFlowOpenAPIV3SchemaByWanwu(ctx context.Context, c *app.RequestContext) {
	var err error
	var req workflow.GetWorkflowDetailRequest
	err = c.BindAndValidate(&req)
	if err != nil {
		invalidParamRequestResponse(c, err.Error())
		return
	}
	schemas, err := appworkflow.SVC.ListWorkFlowOpenAPIV3SchemaByWanwu(ctx, req.WorkflowIds)
	if err != nil {
		invalidParamRequestResponse(c, err.Error())
		return
	}

	c.JSON(consts.StatusOK, schemas)
}

// GetWorkFlowOpenAPIV3SchemaByWanwu 获取workflow openapi v3 schema
// @router /v1/workflow/:workflow_id/schema_by_wanwu [GET]
func GetWorkFlowOpenAPIV3SchemaByWanwu(ctx context.Context, c *app.RequestContext) {
	workflowID := c.Param("workflow_id")
	if workflowID == "" {
		invalidParamRequestResponse(c, "workflow_id empty")
		return
	}
	wfSchema, err := appworkflow.SVC.GetWorkFlowOpenAPIV3SchemaByWanwu(ctx, workflowID)
	if err != nil {
		invalidParamRequestResponse(c, err.Error())
		return
	}

	c.JSON(consts.StatusOK, wfSchema)
}

// OpenAPIRunWorkFlowByWanwu 参考OpenAPIRunFlow
// 0. FIXME 智能体运行该接口，不会在header中带userId、orgId，需要在该方法中设置ctxcache
// 1. 将workflow_id从 body => path
// 2. 将body参数{...} marsharl到req.Parameters上
// 3. ctx中设置WANWU_WORKFLOW_OPENAPI_RUN_RECORD_EXECUTE_HISTORY
// 4. 返回resp.Data unmarshal的结构体
// @router /v1/workflow/:workflow_id/run_by_wanwu [POST]
func OpenAPIRunWorkFlowByWanwu(ctx context.Context, c *app.RequestContext) {
	var err error

	workflowID := c.Param("workflow_id")
	if workflowID == "" {
		invalidParamRequestResponse(c, "workflow_id empty")
		return
	}

	parameters, err := preprocessWorkflowRequestBodyByWanwu(ctx, c)
	if err != nil {
		invalidParamRequestResponse(c, err.Error())
		return
	} else {

	}

	var req workflow.OpenAPIRunFlowRequest

	if err := c.BindAndValidate(&req); err != nil {
		invalidParamRequestResponse(c, err.Error())
		return
	}
	req.Parameters = parameters

	if os.Getenv("WANWU_WORKFLOW_OPENAPI_RUN_SKIP_EXECUTE_HISTORY") == "1" {
		ctx = context.WithValue(ctx, "WANWU_WORKFLOW_OPENAPI_RUN_SKIP_EXECUTE_HISTORY", true)
	}

	resp, tPlan, err := appworkflow.SVC.OpenAPIRunByWanwu(ctx, workflowID, &req)
	if err != nil {
		var se vo.WorkflowError
		if errors.As(err, &se) {
			errResp := &wanwuRunFlowErrorResponse{
				Code: int64(se.OpenAPICode()),
				Msg:  se.Msg(),
			}

			// 仅在非 SKIP 模式时，查询失败节点信息
			if os.Getenv("WANWU_WORKFLOW_OPENAPI_RUN_SKIP_EXECUTE_HISTORY") != "1" {
				// 从 resp 或 DebugURL 获取 executeID
				var executeID int64
				if resp != nil && resp.ExecuteID != nil && *resp.ExecuteID != "" {
					if exeID, parseErr := strconv.ParseInt(*resp.ExecuteID, 10, 64); parseErr == nil {
						executeID = exeID
					}
				} else {
					executeID = parseWorkflowExecuteIDFromDebugURL(se.DebugURL())
				}

				if executeID > 0 {
					errResp.ExecuteID = strconv.FormatInt(executeID, 10)
					errResp.FailedNodes = getWorkflowFailedNodes(ctx, executeID)
				}
			}

			c.JSON(consts.StatusOK, errResp)
			return
		}

		internalServerErrorResponse(ctx, c, err)
		return
	}
	if tPlan == vo.ReturnVariables {
		var respData map[string]any
		if resp.Data != nil {
			if err = sonic.Unmarshal([]byte(*resp.Data), &respData); err != nil {
				logs.CtxErrorf(ctx, "unmarshal resp.Data (%v) err: %v", resp.Data, err)
				c.JSON(consts.StatusOK, resp.Data)
				return
			}
			c.JSON(consts.StatusOK, respData)
			return
		}
	} else {
		c.JSON(consts.StatusOK, resp.Data)
		return
	}

	internalServerErrorResponse(ctx, c, errors.New("empty response"))
}

// OpenAPICreateConversationByWanwu 参考OpenAPICreateConversation
// @router /v1/workflow/conversation/create_by_wanwu [POST]
func OpenAPICreateConversationByWanwu(ctx context.Context, c *app.RequestContext) {
	var err error
	var req workflow.CreateConversationRequest
	err = c.BindAndValidate(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}
	// appID非空，用于对话流调试(appID == workflowID)
	// appID为空，自动生成一个新的，用于应用广场新建对话流
	if req.AppID == nil {
		newAppID, _ := appworkflow.SVC.IDGenerator.GenID(ctx)
		req.AppID = ptr.Of(strconv.FormatInt(newAppID, 10))
	}
	resp, err := appworkflow.SVC.OpenAPICreateConversationByWanwu(ctx, &req)
	if err != nil {
		internalServerErrorResponse(ctx, c, err)
		return
	}
	// 确保ConversationData不为nil
	if resp.ConversationData == nil {
		resp.ConversationData = &workflow.ConversationData{}
	}
	// 确保MetaData不为nil
	if resp.ConversationData.MetaData == nil {
		resp.ConversationData.MetaData = make(map[string]string)
	}
	resp.ConversationData.MetaData["appId"] = req.GetAppID()
	c.JSON(consts.StatusOK, resp)
}

// preprocessWorkflowRequestBodyByWanwu 参考preprocessWorkflowRequestBody
func preprocessWorkflowRequestBodyByWanwu(_ context.Context, c *app.RequestContext) (*string, error) {
	// Read the raw request body
	rawData, err := c.Request.BodyE()
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	// Unmarshal into a temporary map
	var bodyData map[string]interface{}
	if err = sonic.Unmarshal(rawData, &bodyData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal request body: %w", err)
	}

	return ptr.Of(string(rawData)), nil
}

func mustParseInt64(s string) int64 {
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		panic(err)
	}
	return i
}

type wanwuRunFlowErrorResponse struct {
	Code             int64                   `json:"code"`
	Msg              string                  `json:"msg"`
	ExecuteID        string                  `json:"execute_id,omitempty"`
	FailedNodes      []*workflowFailedNode   `json:"failed_nodes,omitempty"`
}

type workflowFailedNode struct {
	NodeID     string                `json:"node_id"`
	NodeName   string                `json:"node_name"`
	NodeType   string                `json:"node_type"`
	NodeStatus workflow.NodeExeStatus `json:"node_status"`
	ErrorInfo  string                `json:"error_info"`
	ErrorLevel *string               `json:"error_level,omitempty"`
	Duration   string                `json:"duration"`
	Input      *string               `json:"input,omitempty"`
	Output     *string               `json:"output,omitempty"`
}

// getWorkflowFailedNodes 查询所有失败节点信息（仅非 SKIP 模式有效）
func getWorkflowFailedNodes(ctx context.Context, executeID int64) []*workflowFailedNode {
	wfExe, err := appworkflow.GetWorkflowDomainSVC().GetExecution(ctx,
		&entity.WorkflowExecution{ID: executeID}, true)
	if err != nil || wfExe == nil {
		return nil
	}

	var nodes []*workflowFailedNode
	for _, nodeExe := range wfExe.NodeExecutions {
		if nodeExe.Status == entity.NodeFailed && nodeExe.ErrorInfo != nil && *nodeExe.ErrorInfo != "" {
			node := &workflowFailedNode{
				NodeID:     nodeExe.NodeID,
				NodeName:   nodeExe.NodeName,
				NodeType:   string(nodeExe.NodeType),
				NodeStatus: workflow.NodeExeStatus(nodeExe.Status),
				ErrorInfo:  *nodeExe.ErrorInfo,
				Duration:   nodeExe.Duration.String(),
				ErrorLevel: nodeExe.ErrorLevel,
				Input:      nodeExe.Input,
				Output:     nodeExe.Output,
			}
			nodes = append(nodes, node)
		}
	}

	return nodes
}

func parseWorkflowExecuteIDFromDebugURL(debugURL string) int64 {
	if debugURL == "" {
		return 0
	}

	u, err := url.Parse(debugURL)
	if err != nil {
		return 0
	}

	executeIDStr := u.Query().Get("execute_id")
	if executeIDStr == "" {
		return 0
	}

	executeID, err := strconv.ParseInt(executeIDStr, 10, 64)
	if err != nil {
		return 0
	}

	return executeID
}
