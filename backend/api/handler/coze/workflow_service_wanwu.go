package coze

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/coze-dev/coze-studio/backend/api/model/workflow"
	appworkflow "github.com/coze-dev/coze-studio/backend/application/workflow"
	workflowModel "github.com/coze-dev/coze-studio/backend/crossdomain/workflow/model"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/entity/vo"
	"github.com/coze-dev/coze-studio/backend/pkg/sonic"
)

// parseInt64Slice 将 []string 转为 []int64
func parseInt64Slice(ss []string) ([]int64, error) {
	result := make([]int64, 0, len(ss))
	for _, s := range ss {
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, nil
}

// parseWorkflowListOptFromBody 从请求 JSON body 中解析 ListWorkflowByWanwuOpt 的额外参数
func parseWorkflowListOptFromBody(c *app.RequestContext) *appworkflow.ListWorkflowByWanwuOpt {
	rawData, err := c.Body()
	if err != nil || len(rawData) == 0 {
		return nil
	}
	var bodyMap map[string]interface{}
	if err := sonic.Unmarshal(rawData, &bodyMap); err != nil {
		return nil
	}

	opt := &appworkflow.ListWorkflowByWanwuOpt{}

	if rawSpaceIDs, ok := bodyMap["space_ids"]; ok {
		switch v := rawSpaceIDs.(type) {
		case []interface{}:
			strs := make([]string, len(v))
			for i, item := range v {
				strs[i] = fmt.Sprint(item)
			}
			opt.SpaceIDs, _ = parseInt64Slice(strs)
		}
	}

	if rawCreatorIDs, ok := bodyMap["creator_ids"]; ok {
		switch v := rawCreatorIDs.(type) {
		case []interface{}:
			strs := make([]string, len(v))
			for i, item := range v {
				strs[i] = fmt.Sprint(item)
			}
			opt.CreatorIDs, _ = parseInt64Slice(strs)
		}
	}

	if len(opt.SpaceIDs) == 0 && len(opt.CreatorIDs) == 0 {
		return nil
	}
	return opt
}

// CreateWorkflowByWanwu 参考CreateWorkflow
// @router /api/workflow_api/create [POST]
func CreateWorkflowByWanwu(ctx context.Context, c *app.RequestContext) {
	var err error
	var req workflow.CreateWorkflowRequest
	err = c.BindAndValidate(&req)
	if err != nil {
		invalidParamRequestResponse(c, err.Error())
		return
	}

	resp, err := appworkflow.SVC.CreateWorkflowByWanwu(ctx, &req)
	if err != nil {
		internalServerErrorResponse(ctx, c, err)
		return
	}

	c.JSON(consts.StatusOK, resp)
}

// ConvertWorkflowByWanwu 参考UpdateWorkflowMeta
// @router /api/workflow_api/convert_by_wanwu [POST]
func ConvertWorkflowByWanwu(ctx context.Context, c *app.RequestContext) {
	var err error
	var req workflow.UpdateWorkflowMetaRequest
	err = c.BindAndValidate(&req)
	if err != nil {
		invalidParamRequestResponse(c, err.Error())
		return
	}
	resp, err := appworkflow.SVC.ConvertWorkflowByWanwu(ctx, &req)

	if err != nil {
		internalServerErrorResponse(ctx, c, err)
		return
	}

	c.JSON(consts.StatusOK, resp)
}

// CopyWorkflowByWanwu 参考CopyWorkflow
// @router /api/workflow_api/copy [POST]
func CopyWorkflowByWanwu(ctx context.Context, c *app.RequestContext) {
	var err error
	var req appworkflow.CopyWorkflowRequest
	var canvasReq appworkflow.ExportWorkflowRequest
	err = c.BindAndValidate(&req)
	if err != nil {
		invalidParamRequestResponse(c, err.Error())
		return
	}
	if req.QType == workflowModel.FromLatestVersion {
		canvasReq.QType = workflowModel.FromLatestVersion
		canvasReq.WorkflowID = req.WorkflowID
		canvasReq.SpaceID = req.SpaceID
		canvas, err := appworkflow.SVC.GetCanvasInfoByWanwu(ctx, &canvasReq)
		if err != nil {
			internalServerErrorResponse(ctx, c, err)
			return
		}
		// 解析原始schema
		var schema vo.Canvas
		if err := sonic.Unmarshal([]byte(*canvas.Data.Workflow.SchemaJSON), &schema); err != nil {
			internalServerErrorResponse(ctx, c, fmt.Errorf("failed to parse schema JSON: %v", err))
			return
		}
		// 清理所有节点的用户自定义参数
		if err := cleanWorkflowSchema(&schema); err != nil {
			internalServerErrorResponse(ctx, c, fmt.Errorf("failed to clean workflow schema: %v", err))
			return
		}
		var schemaStr string
		if schemaStr, err = sonic.MarshalString(schema); err != nil {
			internalServerErrorResponse(ctx, c, fmt.Errorf("failed to marshal workflow schema string: %v", err))
			return
		}
		req.SchemaJSON = &schemaStr
	}
	resp, err := appworkflow.SVC.CopyWorkflowByWanwu(ctx, &req)
	if err != nil {
		internalServerErrorResponse(ctx, c, err)
		return
	}

	c.JSON(consts.StatusOK, resp)
}

// GetWorkFlowListByWanwu 参考GetWorkFlowList
// @router /api/workflow_api/workflow_list_by_wanwu [POST]
func GetWorkFlowListByWanwu(ctx context.Context, c *app.RequestContext) {
	var err error
	var req workflow.GetWorkFlowListRequest
	err = c.BindAndValidate(&req)
	if err != nil {
		invalidParamRequestResponse(c, err.Error())
		return
	}

	// 解析管理员场景的多 space/creator 参数（从 JSON body 中获取额外字段）
	opt := parseWorkflowListOptFromBody(c)
	resp, err := appworkflow.SVC.ListWorkflowByWanwu(ctx, &req, opt)
	if err != nil {
		internalServerErrorResponse(ctx, c, err)
		return
	}
	for _, workflowData := range resp.Data.WorkflowList {
		workflowDefaultIconURL(workflowData)
	}
	c.JSON(consts.StatusOK, resp)
}

// GetWorkFlowSelectByWanwu 参考GetWorkFlowList
// @router /api/workflow_api/workflow_select_by_wanwu [POST]
func GetWorkFlowSelectByWanwu(ctx context.Context, c *app.RequestContext) {
	var err error
	var req workflow.GetWorkFlowListRequest
	err = c.BindAndValidate(&req)
	if err != nil {
		invalidParamRequestResponse(c, err.Error())
		return
	}
	resp, err := appworkflow.SVC.GetWorkFlowSelectByWanwu(ctx, &req)
	if err != nil {
		internalServerErrorResponse(ctx, c, err)
		return
	}
	for _, workflowData := range resp.Data.WorkflowList {
		workflowDefaultIconURL(workflowData)
	}

	c.JSON(consts.StatusOK, resp)
}

// GetExampleWorkFlowListByWanwu 参考GetExampleWorkFlowList，目前去掉其中的example
// @router /api/workflow_api/example_workflow_list [POST]
func GetExampleWorkFlowListByWanwu(ctx context.Context, c *app.RequestContext) {
	var err error
	var req workflow.GetExampleWorkFlowListRequest
	err = c.BindAndValidate(&req)
	if err != nil {
		invalidParamRequestResponse(c, err.Error())
		return
	}

	resp := &workflow.GetExampleWorkFlowListResponse{
		Data: &workflow.WorkFlowListData{
			AuthList:     make([]*workflow.ResourceAuthInfo, 0),
			WorkflowList: make([]*workflow.Workflow, 0),
		},
	}

	c.JSON(consts.StatusOK, resp)
}

// GetWorkflowDetailInfoByWanwu 参考GetWorkflowDetailInfo 替换返回URL
// @router /api/workflow_api/workflow_detail_info [POST]
func GetWorkflowDetailInfoByWanwu(ctx context.Context, c *app.RequestContext) {
	var err error
	var req workflow.GetWorkflowDetailInfoRequest
	err = c.BindAndValidate(&req)
	if err != nil {
		invalidParamRequestResponse(c, err.Error())
		return
	}

	workflowDetailInfoDataList, err := appworkflow.SVC.GetWorkflowDetailInfo(ctx, &req)
	if err != nil {
		internalServerErrorResponse(ctx, c, err)
		return
	}
	for _, workflowDetail := range workflowDetailInfoDataList.List {
		workflowDetailDefaultIconURL(workflowDetail)
	}

	response := map[string]any{
		"data":    workflowDetailInfoDataList,
		"code":    0,
		"message": "",
	}

	c.JSON(consts.StatusOK, response)
}

// GetDraftCanvasInfoByWanwu 参考GetCanvasInfo 获取工作流草稿信息
// @router /api/workflow_api/canvas/draft [POST]
func GetDraftCanvasInfoByWanwu(ctx context.Context, c *app.RequestContext) {
	var req appworkflow.ExportWorkflowRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		invalidParamRequestResponse(c, err.Error())
		return
	}
	req.QType = workflowModel.FromDraft
	resp, err := appworkflow.SVC.GetCanvasInfoByWanwu(ctx, &req)
	if err != nil {
		internalServerErrorResponse(ctx, c, err)
		return
	}
	// 增加返回图标判断
	workflowDefaultIconURL(resp.Data.Workflow)
	c.JSON(consts.StatusOK, resp)
}

// GetLatestVersionCanvasInfoByWanwu 参考GetCanvasInfo 获取已发布工作流最新版本信息
// @router /api/workflow_api/canvas [POST]
func GetLatestVersionCanvasInfoByWanwu(ctx context.Context, c *app.RequestContext) {
	var req appworkflow.ExportWorkflowRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		invalidParamRequestResponse(c, err.Error())
		return
	}
	req.QType = workflowModel.FromLatestVersion
	resp, err := appworkflow.SVC.GetCanvasInfoByWanwu(ctx, &req)
	if err != nil {
		internalServerErrorResponse(ctx, c, err)
		return
	}
	// 增加返回图标判断
	workflowDefaultIconURL(resp.Data.Workflow)
	c.JSON(consts.StatusOK, resp)
}

// RunWorkFlowLatestVersionByWanwu 运行已发布最新版工作流
// @router /v1/workflow/run_by_wanwu [POST]
func RunWorkFlowLatestVersionByWanwu(ctx context.Context, c *app.RequestContext) {
	var err error
	var req workflow.WorkFlowTestRunRequest
	err = c.BindAndValidate(&req)
	if err != nil {
		invalidParamRequestResponse(c, err.Error())
		return
	}

	resp, err := appworkflow.SVC.LatestVersionRunByWanwu(ctx, &req)
	if err != nil {
		internalServerErrorResponse(ctx, c, err)
		return
	}

	c.JSON(consts.StatusOK, resp)
}

// DeleteProjectConversationDefByWanwu 参考DeleteProjectConversationDef
// @router /api/workflow_api/project_conversation/delete_by_wanwu [POST]
func DeleteProjectConversationDefByWanwu(ctx context.Context, c *app.RequestContext) {
	var err error
	var req workflow.DeleteProjectConversationDefRequest
	err = c.BindAndValidate(&req)
	if err != nil {
		invalidParamRequestResponse(c, err.Error())
		return
	}

	resp, err := appworkflow.SVC.DeleteApplicationConversationDefByWanwu(ctx, &req)
	if err != nil {
		internalServerErrorResponse(ctx, c, err)
		return
	}

	c.JSON(consts.StatusOK, resp)
}

// UpdateWorkflowMetaByWanwu 参考UpdateWorkflowMeta
// @router /api/workflow_api/update_meta_by_wanwu [POST]
func UpdateWorkflowMetaByWanwu(ctx context.Context, c *app.RequestContext) {
	var err error
	var req workflow.UpdateWorkflowMetaRequest
	err = c.BindAndValidate(&req)
	if err != nil {
		invalidParamRequestResponse(c, err.Error())
		return
	}
	resp, err := appworkflow.SVC.UpdateWorkflowMetaByWanwu(ctx, &req)

	if err != nil {
		internalServerErrorResponse(ctx, c, err)
		return
	}

	c.JSON(consts.StatusOK, resp)
}

// --- internal ---

func workflowDefaultIconURL(wf *workflow.Workflow) {
	if wf == nil {
		return
	}
	if wf.URL == "" || strings.Contains(wf.URL, "default_workflow_icon.png") {
		switch wf.FlowMode {
		case workflow.WorkflowMode_Workflow:
			wf.URL, _ = url.JoinPath(os.Getenv("WANWU_EXTERNAL_SCHEME")+"://"+os.Getenv("WANWU_EXTERNAL_ENDPOINT"), os.Getenv("WANWU_WORKFLOW_DEFAULT_ICON"))
		case workflow.WorkflowMode_ChatFlow:
			wf.URL, _ = url.JoinPath(os.Getenv("WANWU_EXTERNAL_SCHEME")+"://"+os.Getenv("WANWU_EXTERNAL_ENDPOINT"), os.Getenv("WANWU_CHATFLOW_DEFAULT_ICON"))
		}
	}
}

func workflowDetailDefaultIconURL(wf *workflow.WorkflowDetailInfoData) {
	if wf == nil {
		return
	}
	if wf.Icon == "" || strings.Contains(wf.Icon, "default_workflow_icon.png") {
		switch wf.FlowMode {
		case workflow.WorkflowMode_Workflow:
			wf.Icon, _ = url.JoinPath(os.Getenv("WANWU_EXTERNAL_SCHEME")+"://"+os.Getenv("WANWU_EXTERNAL_ENDPOINT"), os.Getenv("WANWU_WORKFLOW_DEFAULT_ICON"))
		case workflow.WorkflowMode_ChatFlow:
			wf.Icon, _ = url.JoinPath(os.Getenv("WANWU_EXTERNAL_SCHEME")+"://"+os.Getenv("WANWU_EXTERNAL_ENDPOINT"), os.Getenv("WANWU_CHATFLOW_DEFAULT_ICON"))
		}
	}
}
