package coze

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/coze-dev/coze-studio/backend/api/model/workflow"
	appworkflow "github.com/coze-dev/coze-studio/backend/application/workflow"
)

// GetWorkflowVersionListByWanwu 获取工作流版本列表
// @router /api/workflow_api/version_list [POST]
func GetWorkflowVersionListByWanwu(ctx context.Context, c *app.RequestContext) {
	var req appworkflow.GetWorkflowRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		invalidParamRequestResponse(c, err.Error())
		return
	}
	resp, err := appworkflow.SVC.GetWorkflowVersionListByWanwu(ctx, req.WorkflowID)
	if err != nil {
		internalServerErrorResponse(ctx, c, err)
		return
	}
	c.JSON(consts.StatusOK, resp)
}

// UpdateWorkflowVersionDescriptionByWanwu 更新版本描述
// @router /api/workflow_api/version/description [PUT]
func UpdateWorkflowVersionDescriptionByWanwu(ctx context.Context, c *app.RequestContext) {
	var err error
	var req appworkflow.UpdateWorkflowVersionDescriptionRequest
	err = c.BindAndValidate(&req)
	if err != nil {
		invalidParamRequestResponse(c, err.Error())
		return
	}

	resp, err := appworkflow.SVC.UpdateWorkflowVersionDescriptionByWanwu(ctx, &req)
	if err != nil {
		internalServerErrorResponse(ctx, c, err)
		return
	}
	c.JSON(consts.StatusOK, resp)
}

// RollbackWorkflowVersionByWanwu 回滚工作流版本
// @router /api/workflow_api/revert [POST]
func RollbackWorkflowVersionByWanwu(ctx context.Context, c *app.RequestContext) {
	var err error
	var req appworkflow.RollbackWorkflowVersionRequest
	err = c.BindAndValidate(&req)
	if err != nil {
		invalidParamRequestResponse(c, err.Error())
		return
	}
	resp, err := appworkflow.SVC.RollbackWorkflowVersionByWanwu(ctx, &req)
	if err != nil {
		internalServerErrorResponse(ctx, c, err)
		return
	}

	c.JSON(consts.StatusOK, resp)
}

// GetHistorySchemaByWanwu 获取历史工作流schema .
// @router /api/workflow_api/history_schema [POST]
func GetHistorySchemaByWanwu(ctx context.Context, c *app.RequestContext) {
	var err error
	var req workflow.GetHistorySchemaRequest
	err = c.BindAndValidate(&req)
	if err != nil {
		invalidParamRequestResponse(c, err.Error())
		return
	}

	resp, err := appworkflow.SVC.GetWorkflowVersionSchemaByWanwu(ctx, &req)
	if err != nil {
		internalServerErrorResponse(ctx, c, err)
		return
	}

	c.JSON(consts.StatusOK, resp)
}

// MGetWorkflowLatestVersionByWanwu 批量获取工作流最新版本列表
// @router /api/workflow_api/latest_version_list [POST]
func MGetWorkflowLatestVersionByWanwu(ctx context.Context, c *app.RequestContext) {
	var req appworkflow.MGetWorkflowLatestVersionRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		invalidParamRequestResponse(c, err.Error())
		return
	}
	resp, err := appworkflow.SVC.MGetWorkflowLatestVersionByWanwu(ctx, &req)
	if err != nil {
		internalServerErrorResponse(ctx, c, err)
		return
	}
	c.JSON(consts.StatusOK, resp)
}
