package workflow

import (
	"context"
	"fmt"
	"path"
	"runtime/debug"
	"strconv"

	"github.com/cloudwego/eino/schema"
	"github.com/coze-dev/coze-studio/backend/api/model/workflow"
	"github.com/coze-dev/coze-studio/backend/application/base/ctxutil"
	"github.com/coze-dev/coze-studio/backend/bizpkg/debugutil"
	workflowModel "github.com/coze-dev/coze-studio/backend/crossdomain/workflow/model"
	user_entity "github.com/coze-dev/coze-studio/backend/domain/user/entity"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/entity"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/entity/vo"
	"github.com/coze-dev/coze-studio/backend/pkg/ctxcache"
	"github.com/coze-dev/coze-studio/backend/pkg/errorx"
	"github.com/coze-dev/coze-studio/backend/pkg/i18n"
	"github.com/coze-dev/coze-studio/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-studio/backend/pkg/safego"
	"github.com/coze-dev/coze-studio/backend/pkg/sonic"
	"github.com/coze-dev/coze-studio/backend/types/consts"
	"github.com/coze-dev/coze-studio/backend/types/errno"
	"github.com/getkin/kin-openapi/openapi3"
)

func (w *ApplicationService) ListWorkFlowOpenAPIV3SchemaByWanwu(ctx context.Context, workflowIDs []string) (
	_ []map[string]any, err error,
) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			err = safego.NewPanicErr(panicErr, debug.Stack())
		}

		if err != nil {
			err = vo.WrapIfNeeded(errno.ErrWorkflowExecuteFail, err, errorx.KV("cause", vo.UnwrapRootErr(err).Error()))
		}
	}()

	if len(workflowIDs) == 0 {
		return nil, fmt.Errorf("workflowIDs empty")
	}
	var ids []int64
	for _, workflowID := range workflowIDs {
		ids = append(ids, mustParseInt64(workflowID))
	}

	wfs, _, err := GetWorkflowDomainSVC().MGet(ctx, &vo.MGetPolicy{MetaQuery: vo.MetaQuery{IDs: ids}, QType: workflowModel.FromLatestVersion})
	if err != nil {
		return nil, err
	}
	var rets []map[string]any
	for _, wf := range wfs {
		wfSchema, err := workflowOpenAPIV3Schema(wf)
		if err != nil {
			return nil, err
		}
		rets = append(rets, wfSchema)
	}
	return rets, nil
}

func (w *ApplicationService) GetWorkFlowOpenAPIV3SchemaByWanwu(ctx context.Context, workflowID string) (
	_ map[string]any, err error,
) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			err = safego.NewPanicErr(panicErr, debug.Stack())
		}

		if err != nil {
			err = vo.WrapIfNeeded(errno.ErrWorkflowExecuteFail, err, errorx.KV("cause", vo.UnwrapRootErr(err).Error()))
		}
	}()

	wf, err := GetWorkflowDomainSVC().Get(ctx, &vo.GetPolicy{ID: mustParseInt64(workflowID), QType: workflowModel.FromLatestVersion})
	if err != nil {
		return nil, err
	}
	return workflowOpenAPIV3Schema(wf)
}

func workflowOpenAPIV3Schema(wf *entity.Workflow) (map[string]any, error) {
	inputSchema, err := workflowParamsToSchema(wf.InputParams)
	if err != nil {
		return nil, fmt.Errorf("workflow(%v) input params to openapi v3 schema err: %v", wf.ID, err)
	}
	outputSchema, err := workflowParamsToSchema(wf.OutputParams)
	if err != nil {
		return nil, fmt.Errorf("workflow(%v) output params to openapi v3 schema err: %v", wf.ID, err)
	}

	var input any = inputSchema
	if inputSchema == nil {
		input = map[string]string{
			"type": "object",
		}
	}
	var output any = outputSchema
	if outputSchema == nil {
		output = map[string]string{
			"type": "object",
		}
	}

	return map[string]any{
		"openapi": "3.0.0",
		"info": map[string]string{
			"title":       wf.Name,
			"version":     "1.0.0",
			"description": wf.Desc,
		},
		"servers": []map[string]string{
			{
				"url": "http://workflow-wanwu:8999/v1",
			},
		},
		"paths": map[string]any{
			path.Join("/workflow", strconv.Itoa(int(wf.ID)), "/run_by_wanwu"): map[string]any{
				"post": map[string]any{
					"summary":     wf.Name,
					"operationId": "action_" + wf.Name,
					"description": wf.Desc,
					"parameters": []map[string]any{
						{
							"in":   "header",
							"name": "Content-Type",
							"schema": map[string]string{
								"type":    "string",
								"example": "application/json",
							},
							"required": true,
						},
					},
					"requestBody": map[string]any{
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": input,
							},
						},
					},
					"responses": map[string]any{
						"200": map[string]any{
							"description": "请求成功时的结果",
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": output,
								},
							},
						},
						"default": map[string]any{
							"description": "请求失败时的错误信息",
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]string{
										"type": "object",
									},
								},
							},
						},
					},
				},
			},
		},
	}, nil
}

func workflowParamsToSchema(params []*vo.NamedTypeInfo) (*openapi3.Schema, error) {
	var paramsMap map[string]*schema.ParameterInfo
	for _, param := range params {
		paramInfo, err := param.ToParameterInfo()
		if err != nil {
			return nil, err
		}
		if paramsMap == nil {
			paramsMap = make(map[string]*schema.ParameterInfo)
		}
		paramsMap[param.Name] = paramInfo
	}
	return schema.NewParamsOneOfByParams(paramsMap).ToOpenAPIV3()
}

// OpenAPIRunByWanwu 参考OpenAPIRun
// 0. FIXME 智能体运行该接口，不会在header中带userId、orgId，需要在该方法中设置ctxcache
// 1. 去掉api auth、user check等业务逻辑
// 2. 去掉appID、agentID、connectorID等业务逻辑
// 3. 将必须publish才能执行的workflow，改为可以执行draft
func (w *ApplicationService) OpenAPIRunByWanwu(ctx context.Context, workflowID string, req *workflow.OpenAPIRunFlowRequest) (
	_ *workflow.OpenAPIRunFlowResponse, _ vo.TerminatePlan, err error,
) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			err = safego.NewPanicErr(panicErr, debug.Stack())
		}

		if err != nil {
			err = vo.WrapIfNeeded(errno.ErrWorkflowExecuteFail, err, errorx.KV("cause", vo.UnwrapRootErr(err).Error()))
		}
	}()

	// apiKeyInfo := ctxutil.GetApiAuthFromCtx(ctx)
	// userID := apiKeyInfo.UserID

	parameters := make(map[string]any)
	if req.Parameters != nil {
		err := sonic.UnmarshalString(*req.Parameters, &parameters)
		if err != nil {
			return nil, "", vo.WrapError(errno.ErrInvalidParameter, err)
		}
	}

	meta, err := GetWorkflowDomainSVC().Get(ctx, &vo.GetPolicy{
		ID:       mustParseInt64(workflowID),
		MetaOnly: true,
		QType:    workflowModel.FromLatestVersion,
	})
	if err != nil {
		return nil, "", err
	}

	// 设置ctxcache
	if ctxutil.GetUserSessionFromCtx(ctx) == nil {
		ctxcache.Store(ctx, consts.SessionDataKeyInCtx, &user_entity.Session{
			UserID: meta.CreatorID,
			Locale: string(i18n.GetLocale(ctx)),
		})
		ctxcache.Store(ctx, "X-Org-Id", strconv.Itoa(int(meta.SpaceID)))
	}

	if meta.LatestPublishedVersion == nil {
		return nil, "", vo.NewError(errno.ErrWorkflowNotPublished)
	}

	// if err = checkUserSpace(ctx, userID, meta.SpaceID); err != nil {
	// 	return nil, err
	// }

	// var appID, agentID *int64
	// if req.IsSetAppID() {
	// 	appID = ptr.Of(mustParseInt64(req.GetAppID()))
	// } else if req.IsSetProjectID() {
	// 	appID = ptr.Of(mustParseInt64(req.GetProjectID()))
	// }
	// if req.IsSetBotID() {
	// 	agentID = ptr.Of(mustParseInt64(req.GetBotID()))
	// }

	// var connectorID int64
	// if req.IsSetConnectorID() {
	// 	connectorID = mustParseInt64(req.GetConnectorID())
	// }

	// if connectorID != consts.WebSDKConnectorID {
	// 	connectorID = apiKeyInfo.ConnectorID
	// }

	exeCfg := workflowModel.ExecuteConfig{
		ID:       meta.ID,
		From:     workflowModel.FromLatestVersion,
		Version:  *meta.LatestPublishedVersion,
		Operator: meta.CreatorID,
		Mode:     workflowModel.ExecuteModeRelease,
		AppID:    ptr.Of(meta.ID),
		// AgentID:       agentID,
		ConnectorID:   consts.APIConnectorID,
		ConnectorUID:  strconv.FormatInt(meta.CreatorID, 10),
		InputFailFast: true,
		BizType:       workflowModel.BizTypeWorkflow,
	}

	// if exeCfg.AppID != nil && exeCfg.AgentID != nil {
	// 	return nil, errors.New("project_id and bot_id cannot be set at the same time")
	// }

	if req.GetIsAsync() {
		exeCfg.SyncPattern = workflowModel.SyncPatternAsync
		exeCfg.TaskType = workflowModel.TaskTypeBackground
		exeID, err := GetWorkflowDomainSVC().AsyncExecute(ctx, exeCfg, parameters)
		if err != nil {
			return nil, "", err
		}

		return &workflow.OpenAPIRunFlowResponse{
			ExecuteID: ptr.Of(strconv.FormatInt(exeID, 10)),
			DebugUrl:  ptr.Of(debugutil.GetWorkflowDebugURL(ctx, meta.ID, meta.SpaceID, exeID)),
		}, "", nil
	}

	exeCfg.SyncPattern = workflowModel.SyncPatternSync
	exeCfg.TaskType = workflowModel.TaskTypeForeground
	wfExe, tPlan, err := GetWorkflowDomainSVC().SyncExecute(ctx, exeCfg, parameters)
	if err != nil {
		return nil, "", err
	}

	if wfExe.Status == entity.WorkflowInterrupted {
		return nil, "", vo.NewError(errno.ErrInterruptNotSupported)
	}

	data := wfExe.Output
	return &workflow.OpenAPIRunFlowResponse{
		Data:      data,
		ExecuteID: ptr.Of(strconv.FormatInt(wfExe.ID, 10)),
		DebugUrl:  ptr.Of(debugutil.GetWorkflowDebugURL(ctx, meta.ID, wfExe.SpaceID, wfExe.ID)),
		Token:     ptr.Of(wfExe.TokenInfo.InputTokens + wfExe.TokenInfo.OutputTokens),
		Cost:      ptr.Of("0.00000"),
	}, tPlan, nil
}
