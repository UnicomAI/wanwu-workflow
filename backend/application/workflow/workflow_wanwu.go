package workflow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/coze-dev/coze-studio/backend/api/model/playground"
	resource "github.com/coze-dev/coze-studio/backend/api/model/resource/common"
	"github.com/coze-dev/coze-studio/backend/api/model/workflow"
	"github.com/coze-dev/coze-studio/backend/application/base/ctxutil"
	"github.com/coze-dev/coze-studio/backend/application/user"
	model "github.com/coze-dev/coze-studio/backend/crossdomain/workflow/model"
	workflowModel "github.com/coze-dev/coze-studio/backend/crossdomain/workflow/model"
	search "github.com/coze-dev/coze-studio/backend/domain/search/entity"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/entity/vo"
	"github.com/coze-dev/coze-studio/backend/pkg/errorx"
	"github.com/coze-dev/coze-studio/backend/pkg/i18n"
	"github.com/coze-dev/coze-studio/backend/pkg/lang/maps"
	"github.com/coze-dev/coze-studio/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-studio/backend/pkg/lang/slices"
	"github.com/coze-dev/coze-studio/backend/pkg/lang/ternary"
	"github.com/coze-dev/coze-studio/backend/pkg/logs"
	"github.com/coze-dev/coze-studio/backend/pkg/safego"
	"github.com/coze-dev/coze-studio/backend/pkg/sonic"
	"github.com/coze-dev/coze-studio/backend/types/consts"
	"github.com/coze-dev/coze-studio/backend/types/errno"
	"github.com/go-resty/resty/v2"
	xmaps "golang.org/x/exp/maps"
)

// CreateWorkflowByWanwu 参考CreateWorkflow，适配了chatflow
// 1. 调换顺序，先创建workflow，再创建conversation
// 2. conversation名，默认为工作流(chatflow)名
// 3. conversation的app id，为创建的工作流id，这样conversation对应唯一的工作流(chatflow)
func (w *ApplicationService) CreateWorkflowByWanwu(ctx context.Context, req *workflow.CreateWorkflowRequest) (
	_ *workflow.CreateWorkflowResponse, err error,
) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			err = safego.NewPanicErr(panicErr, debug.Stack())
		}

		if err != nil {
			err = vo.WrapIfNeeded(errno.ErrWorkflowOperationFail, err, errorx.KV("cause", vo.UnwrapRootErr(err).Error()))
		}
	}()

	uID := ctxutil.MustGetUIDFromCtx(ctx)
	spaceID := mustParseInt64(req.GetSpaceID())
	if err := checkUserSpace(ctx, uID, spaceID); err != nil {
		return nil, err
	}

	wf := &vo.MetaCreate{
		CreatorID:        uID,
		SpaceID:          spaceID,
		ContentType:      workflow.WorkFlowType_User,
		Name:             req.Name,
		Desc:             req.Desc,
		IconURI:          req.IconURI,
		AppID:            parseInt64(req.ProjectID),
		Mode:             ternary.IFElse(req.IsSetFlowMode(), req.GetFlowMode(), workflow.WorkflowMode_Workflow),
		InitCanvasSchema: vo.GetDefaultInitCanvasJsonSchema(i18n.GetLocale(ctx)),
	}
	if req.IsSetFlowMode() && req.GetFlowMode() == workflow.WorkflowMode_ChatFlow {
		wf.InitCanvasSchema = vo.GetDefaultInitCanvasJsonSchemaChat(i18n.GetLocale(ctx), req.Name)
	}

	id, err := GetWorkflowDomainSVC().Create(ctx, wf)
	if err != nil {
		return nil, err
	}

	err = PublishWorkflowResource(ctx, id, ptr.Of(int32(wf.Mode)), search.Created, &search.ResourceDocument{
		Name:          &wf.Name,
		APPID:         wf.AppID,
		SpaceID:       &wf.SpaceID,
		OwnerID:       &wf.CreatorID,
		PublishStatus: ptr.Of(resource.PublishStatus_UnPublished),
		CreateTimeMS:  ptr.Of(time.Now().UnixMilli()),
	})
	if err != nil {
		return nil, vo.WrapError(errno.ErrNotifyWorkflowResourceChangeErr, err)
	}

	if req.IsSetFlowMode() && req.GetFlowMode() == workflow.WorkflowMode_ChatFlow {
		_, err := GetWorkflowDomainSVC().CreateDraftConversationTemplate(ctx, &vo.CreateConversationTemplateMeta{
			AppID:   id,
			UserID:  uID,
			SpaceID: spaceID,
			Name:    req.Name,
		})
		if err != nil {
			return nil, err
		}
		// 如果是对话流还需要创建chat_flow_role
		_, err = GetWorkflowDomainSVC().CreateChatFlowRole(ctx, &vo.ChatFlowRoleCreate{
			WorkflowID: id,
			CreatorID:  uID,
		})
		if err != nil {
			return nil, err
		}
	}

	return &workflow.CreateWorkflowResponse{
		Data: &workflow.CreateWorkflowData{
			WorkflowID: strconv.FormatInt(id, 10),
		},
	}, nil
}

// UpdateWorkflowMetaByWanwu 参考UpdateWorkflowMeta
// 1. 如果是转换成chatflow模式，且没有对话模板，则创建一个默认的对话模板
func (w *ApplicationService) UpdateWorkflowMetaByWanwu(ctx context.Context, req *workflow.UpdateWorkflowMetaRequest) (
	_ *workflow.UpdateWorkflowMetaResponse, err error,
) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			err = safego.NewPanicErr(panicErr, debug.Stack())
		}

		if err != nil {
			err = vo.WrapIfNeeded(errno.ErrWorkflowOperationFail, err, errorx.KV("cause", vo.UnwrapRootErr(err).Error()))
		}
	}()

	if err := checkUserSpace(ctx, ctxutil.MustGetUIDFromCtx(ctx), mustParseInt64(req.GetSpaceID())); err != nil {
		return nil, err
	}

	workflowID := mustParseInt64(req.GetWorkflowID())
	if req.IsSetFlowMode() && req.GetFlowMode() == workflow.WorkflowMode_ChatFlow {
		def, err := w.ListApplicationConversationDef(ctx, &workflow.ListProjectConversationRequest{
			ProjectID:    strconv.FormatInt(workflowID, 10),
			CreateMethod: workflow.CreateMethod_ManualCreate,
			CreateEnv:    workflow.CreateEnv_Draft,
			Limit:        1000,
			SpaceID:      req.GetSpaceID(),
		})
		if err != nil {
			return nil, err
		}
		if len(def.Data) == 0 {
			wf, err := GetWorkflowDomainSVC().Get(ctx, &vo.GetPolicy{
				ID:       mustParseInt64(req.GetWorkflowID()),
				MetaOnly: true,
			})
			if err != nil {
				return nil, err
			}
			_, err = GetWorkflowDomainSVC().CreateDraftConversationTemplate(ctx, &vo.CreateConversationTemplateMeta{
				AppID:   workflowID,
				UserID:  ctxutil.MustGetUIDFromCtx(ctx),
				SpaceID: mustParseInt64(req.GetSpaceID()),
				Name:    wf.Name,
			})
			if err != nil {
				return nil, err
			}
		}
		oldRole, err := GetWorkflowDomainSVC().GetChatFlowRole(ctx, workflowID, "")
		if err != nil {
			return nil, err
		}
		if oldRole == nil {
			// 如果是对话流并且role不存在还需要创建chat_flow_role
			_, err = GetWorkflowDomainSVC().CreateChatFlowRole(ctx, &vo.ChatFlowRoleCreate{
				WorkflowID: workflowID,
				CreatorID:  ctxutil.MustGetUIDFromCtx(ctx),
			})
			if err != nil {
				return nil, err
			}
		}
	}

	err = GetWorkflowDomainSVC().UpdateMeta(ctx, mustParseInt64(req.GetWorkflowID()), &vo.MetaUpdate{
		Name:         req.Name,
		Desc:         req.Desc,
		IconURI:      req.IconURI,
		WorkflowMode: req.FlowMode,
	})
	if err != nil {
		return nil, err
	}
	// 转换工作流 -> 对话流时，domain 层 adaptToChatFlow 会经历 vo.Canvas 的反序列化/序列化。
	// HTTP 节点鉴权参数上的顶层 param.type 不是 vo.Param 的字段，会在这一步被抹掉，
	// 前端初始化 HTTP 鉴权表单时会报 Unknown variable DTO Type:undefined:undefined。
	// 这里在 wanwu 侧做一次 schema 回写补齐，避免修改 coze 通用逻辑。
	if req.IsSetFlowMode() && req.GetFlowMode() == workflow.WorkflowMode_ChatFlow {
		wfEntity, err := GetWorkflowDomainSVC().Get(ctx, &vo.GetPolicy{
			ID:    workflowID,
			QType: workflowModel.FromDraft,
		})
		if err != nil {
			return nil, err
		}
		patchedSchema, err := patchWANWUHTTPAuthParamTypeInSchemaString(wfEntity.Canvas)
		if err != nil {
			return nil, err
		}
		if patchedSchema != wfEntity.Canvas {
			if err := GetWorkflowDomainSVC().Save(ctx, workflowID, patchedSchema); err != nil {
				return nil, err
			}
		}
	}

	safego.Go(ctx, func() {
		err := PublishWorkflowResource(ctx, workflowID, nil, search.Updated, &search.ResourceDocument{
			Name:         req.Name,
			UpdateTimeMS: ptr.Of(time.Now().UnixMilli()),
		})
		if err != nil {
			logs.CtxErrorf(ctx, "publish update workflow resource failed, workflowID: %d, err: %v", workflowID, err)
		}
	})

	return &workflow.UpdateWorkflowMetaResponse{}, nil
}

// CopyWorkflowByWanwu 参考CopyWorkflow，适配了chatflow
func (w *ApplicationService) CopyWorkflowByWanwu(ctx context.Context, req *CopyWorkflowRequest) (
	resp *workflow.CopyWorkflowResponse, err error,
) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			err = safego.NewPanicErr(panicErr, debug.Stack())
		}

		if err != nil {
			err = vo.WrapIfNeeded(errno.ErrWorkflowOperationFail, err, errorx.KV("cause", vo.UnwrapRootErr(err).Error()))
		}
	}()
	spaceID, err := strconv.ParseInt(req.SpaceID, 10, 64)
	if err != nil {
		return nil, err
	}

	if err = checkUserSpace(ctx, ctxutil.MustGetUIDFromCtx(ctx), spaceID); err != nil {
		return nil, err
	}
	workflowID, err := strconv.ParseInt(req.WorkflowID, 10, 64)
	if err != nil {
		return nil, err
	}
	policy := vo.CopyWorkflowPolicy{
		TargetSpaceID:            &spaceID,
		ModifiedCanvasSchema:     req.SchemaJSON,
		ShouldModifyWorkflowName: true,
	}
	wf, err := w.copyWorkflow(ctx, workflowID, policy)
	if err != nil {
		return nil, err
	}
	// 对话流的复制需要创建对话
	if wf.Mode == workflow.WorkflowMode_ChatFlow {
		_, err := GetWorkflowDomainSVC().CreateDraftConversationTemplate(ctx, &vo.CreateConversationTemplateMeta{
			AppID:   wf.ID,
			UserID:  wf.CreatorID,
			SpaceID: spaceID,
			Name:    wf.Name,
		})
		if err != nil {
			return nil, err
		}
	}
	return &workflow.CopyWorkflowResponse{
		Data: &workflow.CopyWorkflowData{
			WorkflowID: strconv.FormatInt(wf.ID, 10),
		},
	}, err
}

// ListWorkflowByWanwu 参考ListWorkflow
// 1. size上限 300 -> 99999
// 2. space_id非必须
// 3. login_user_create为true时筛选workflow.CreatorID为当前用户
func (w *ApplicationService) ListWorkflowByWanwu(ctx context.Context, req *workflow.GetWorkFlowListRequest) (
	_ *workflow.GetWorkFlowListResponse, err error,
) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			err = safego.NewPanicErr(panicErr, debug.Stack())
		}

		if err != nil {
			err = vo.WrapIfNeeded(errno.ErrWorkflowOperationFail, err, errorx.KV("cause", vo.UnwrapRootErr(err).Error()))
		}
	}()

	option := vo.MetaQuery{}

	if req.GetPage() <= 0 || req.GetSize() <= 0 || req.GetSize() > 99999 {
		return nil, fmt.Errorf("the number of page or size must be greater than 0, and the size must be greater than 0 and less than 99999")
	}
	option.Page = &vo.Page{
		Page: req.GetPage(),
		Size: req.GetSize(),
	}

	userID := ctxutil.MustGetUIDFromCtx(ctx)
	if req.GetSpaceID() != "" {
		spaceID := mustParseInt64(req.GetSpaceID())
		if err := checkUserSpace(ctx, userID, spaceID); err != nil {
			return nil, err
		}
		option.SpaceID = ptr.Of(spaceID)
	}

	if len(req.GetName()) > 0 {
		option.Name = req.Name
	}

	if len(req.GetWorkflowIds()) > 0 {
		ids, err := slices.TransformWithErrorCheck[string, int64](req.GetWorkflowIds(), func(s string) (int64, error) {
			return strconv.ParseInt(s, 10, 64)
		})
		if err != nil {
			return nil, err
		}
		option.IDs = ids
	}

	if req.IsSetFlowMode() && req.GetFlowMode() != workflow.WorkflowMode_All {
		option.Mode = ptr.Of(workflowModel.WorkflowMode(req.GetFlowMode()))
	}

	status := req.GetStatus()
	var qType workflowModel.Locator
	if status == workflow.WorkFlowListStatus_UnPublished {
		option.PublishStatus = ptr.Of(vo.UnPublished)
		qType = workflowModel.FromDraft
	} else if status == workflow.WorkFlowListStatus_HadPublished {
		option.PublishStatus = ptr.Of(vo.HasPublished)
		qType = workflowModel.FromLatestVersion
	}

	wfs, total, err := GetWorkflowDomainSVC().MGet(ctx, &vo.MGetPolicy{
		MetaQuery: option,
		QType:     qType,
		MetaOnly:  false,
	})
	if err != nil {
		return nil, err
	}

	response := &workflow.GetWorkFlowListResponse{
		Data: &workflow.WorkFlowListData{
			AuthList:     make([]*workflow.ResourceAuthInfo, 0),
			WorkflowList: make([]*workflow.Workflow, 0, len(wfs)),
		},
	}

	wf2CreatorID := make(map[int64]string)
	workflowList := make([]*workflow.Workflow, 0, len(wfs))
	for _, w := range wfs {

		if req.GetLoginUserCreate() && w.CreatorID != userID {
			continue
		}

		wf2CreatorID[w.ID] = strconv.FormatInt(w.CreatorID, 10)
		ww := &workflow.Workflow{
			WorkflowID:       strconv.FormatInt(w.ID, 10),
			Name:             w.Name,
			Desc:             w.Desc,
			IconURI:          w.IconURI,
			URL:              w.IconURL,
			CreateTime:       w.CreatedAt.Unix(),
			Type:             w.ContentType,
			SchemaType:       workflow.SchemaType_FDL,
			Tag:              w.Tag,
			TemplateAuthorID: ptr.Of(strconv.FormatInt(w.AuthorID, 10)),
			SpaceID:          ptr.Of(strconv.FormatInt(w.SpaceID, 10)),
			PluginID: func() string {
				if status == workflow.WorkFlowListStatus_UnPublished {
					return "0"
				}
				return strconv.FormatInt(w.ID, 10)
			}(),
			Creator: &workflow.Creator{
				ID:   strconv.FormatInt(w.CreatorID, 10),
				Self: ternary.IFElse[bool](w.CreatorID == ptr.From(ctxutil.GetUIDFromCtx(ctx)), true, false),
			},
			FlowMode: w.Mode,
		}

		if len(req.Checker) > 0 && status == workflow.WorkFlowListStatus_HadPublished {
			ww.CheckResult, err = GetWorkflowDomainSVC().WorkflowSchemaCheck(ctx, w, req.Checker)
			if err != nil {
				return nil, err
			}
		}

		if qType == workflowModel.FromDraft {
			ww.UpdateTime = w.DraftMeta.Timestamp.Unix()
		} else if qType == workflowModel.FromLatestVersion || qType == workflowModel.FromSpecificVersion {
			ww.UpdateTime = w.VersionMeta.VersionCreatedAt.Unix()
		} else if w.UpdatedAt != nil {
			ww.UpdateTime = w.UpdatedAt.Unix()
		}

		startNode := &workflow.Node{
			NodeID:    "100001",
			NodeName:  "start-node",
			NodeParam: &workflow.NodeParam{InputParameters: make([]*workflow.Parameter, 0)},
		}

		for _, in := range w.InputParams {
			param, err := toWorkflowParameter(in)
			if err != nil {
				return nil, err
			}
			startNode.NodeParam.InputParameters = append(startNode.NodeParam.InputParameters, param)
		}

		ww.StartNode = startNode

		auth := &workflow.ResourceAuthInfo{
			WorkflowID: strconv.FormatInt(w.ID, 10),
			UserID:     strconv.FormatInt(w.CreatorID, 10),
			Auth:       &workflow.ResourceActionAuth{CanEdit: true, CanDelete: true, CanCopy: true},
		}
		workflowList = append(workflowList, ww)
		response.Data.AuthList = append(response.Data.AuthList, auth)
	}

	userBasicInfoResponse, err := user.UserApplicationSVC.MGetUserBasicInfo(ctx, &playground.MGetUserBasicInfoRequest{UserIds: slices.Unique(xmaps.Values(wf2CreatorID))})
	if err != nil {
		return nil, err
	}

	for _, w := range workflowList {
		if u, ok := userBasicInfoResponse.UserBasicInfoMap[w.Creator.ID]; ok {
			w.Creator.Name = u.Username
			w.Creator.AvatarURL = u.UserAvatar
		}
	}

	response.Data.WorkflowList = workflowList
	response.Data.Total = total

	return response, nil
}

func (w *ApplicationService) GetWorkFlowSelectByWanwu(ctx context.Context, req *workflow.GetWorkFlowListRequest) (
	_ *workflow.GetWorkFlowListResponse, err error,
) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			err = safego.NewPanicErr(panicErr, debug.Stack())
		}

		if err != nil {
			err = vo.WrapIfNeeded(errno.ErrWorkflowOperationFail, err, errorx.KV("cause", vo.UnwrapRootErr(err).Error()))
		}
	}()
	userID := ctxutil.MustGetUIDFromCtx(ctx)
	status := req.GetStatus()
	// 调用ListWorkflowByWanwu需要将status置为nil 查询所有的workflow列表
	// 设置size大小为99999
	req.Status = nil
	sizeValue := int32(99999)
	req.Size = &sizeValue
	resp, err := w.ListWorkflowByWanwu(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(resp.Data.WorkflowList) == 0 {
		return &workflow.GetWorkFlowListResponse{
			Data: &workflow.WorkFlowListData{
				AuthList:     make([]*workflow.ResourceAuthInfo, 0),
				WorkflowList: make([]*workflow.Workflow, 0),
			},
		}, nil
	}
	// 调用BFF的CallBack接口获取发布的workflow IDs
	ret := &cozeWorkflowSelectByWanwuResp{}
	if resp, err := resty.New().
		R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetQueryParams(map[string]string{
			"userId": strconv.FormatInt(userID, 10),
			"orgId":  req.GetSpaceID(),
		}).
		SetResult(ret).
		Get(os.Getenv("WANWU_CALLBACK_WORKFLOW_LIST_URL")); err != nil {
		return nil, err
	} else if resp.StatusCode() >= 300 {
		return nil, fmt.Errorf("http request failed with status code: %d", resp.StatusCode())
	}
	// 调用BFF的CallBack接口获取发布的chatflow IDs
	chatRet := &cozeWorkflowSelectByWanwuResp{}
	if resp, err := resty.New().
		R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetQueryParams(map[string]string{
			"userId": strconv.FormatInt(userID, 10),
			"orgId":  req.GetSpaceID(),
		}).
		SetResult(chatRet).
		Get(os.Getenv("WANWU_CALLBACK_CHATFLOW_LIST_URL")); err != nil {
		return nil, err
	} else if resp.StatusCode() >= 300 {
		return nil, fmt.Errorf("http request failed with status code: %d", resp.StatusCode())
	}
	ret.Data.List = append(ret.Data.List, chatRet.Data.List...)
	// 获取已发布的workflow ID集合
	publishedWorkflowIDs := make(map[string]bool)
	for _, wf := range ret.Data.List {
		publishedWorkflowIDs[wf.AppId] = true
	}
	// 过滤WorkflowList
	var filteredWorkflows []*workflow.Workflow
	for _, wf := range resp.Data.WorkflowList {
		isPublished := publishedWorkflowIDs[wf.WorkflowID]
		if status == workflow.WorkFlowListStatus_HadPublished && isPublished {
			filteredWorkflows = append(filteredWorkflows, wf)
		}
		if status == workflow.WorkFlowListStatus_UnPublished && !isPublished {
			wf.PluginID = "0"
			filteredWorkflows = append(filteredWorkflows, wf)
		}
	}
	// 过滤AuthList
	var filteredAuthList []*workflow.ResourceAuthInfo
	for _, wf := range resp.Data.AuthList {
		isPublished := publishedWorkflowIDs[wf.WorkflowID]
		if status == workflow.WorkFlowListStatus_HadPublished && isPublished {
			filteredAuthList = append(filteredAuthList, wf)
		}
		if status == workflow.WorkFlowListStatus_UnPublished && !isPublished {
			filteredAuthList = append(filteredAuthList, wf)
		}
	}
	return &workflow.GetWorkFlowListResponse{
		Data: &workflow.WorkFlowListData{
			AuthList:     filteredAuthList,
			WorkflowList: filteredWorkflows,
			Total:        int64(len(filteredWorkflows)),
		},
	}, nil
}

// LatestVersionRunByWanwu 参考 TestRun 执行已发布的工作流
func (w *ApplicationService) LatestVersionRunByWanwu(ctx context.Context, req *workflow.WorkFlowTestRunRequest) (_ *workflow.WorkFlowTestRunResponse, err error) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			err = safego.NewPanicErr(panicErr, debug.Stack())
		}

		if err != nil {
			err = vo.WrapIfNeeded(errno.ErrWorkflowExecuteFail, err, errorx.KV("cause", vo.UnwrapRootErr(err).Error()))
		}
	}()

	uID := ctxutil.MustGetUIDFromCtx(ctx)

	if err := checkUserSpace(ctx, uID, mustParseInt64(req.GetSpaceID())); err != nil {
		return nil, err
	}

	var appID, agentID *int64
	if req.IsSetProjectID() {
		appID = ptr.Of(mustParseInt64(req.GetProjectID()))
	}
	if req.IsSetBotID() {
		agentID = ptr.Of(mustParseInt64(req.GetBotID()))
	}

	exeCfg := workflowModel.ExecuteConfig{
		ID:           mustParseInt64(req.GetWorkflowID()),
		From:         workflowModel.FromLatestVersion, // 这里是最新发布版本
		CommitID:     req.GetCommitID(),
		Operator:     uID,
		Mode:         workflowModel.ExecuteModeRelease, // 发布模式（需要将应用广场运行的工作流中的数据库节点等资源指向正式环境）
		AppID:        appID,
		AgentID:      agentID,
		ConnectorID:  consts.CozeConnectorID,
		ConnectorUID: strconv.FormatInt(uID, 10),
		TaskType:     workflowModel.TaskTypeForeground,
		SyncPattern:  workflowModel.SyncPatternAsync,
		BizType:      workflowModel.BizTypeWorkflow,
		Cancellable:  true,
	}

	if exeCfg.AppID != nil && exeCfg.AgentID != nil {
		return nil, errors.New("project_id and bot_id cannot be set at the same time")
	}

	exeID, err := GetWorkflowDomainSVC().AsyncExecute(ctx, exeCfg, maps.ToAnyValue(req.Input))
	if err != nil {
		return nil, err
	}

	return &workflow.WorkFlowTestRunResponse{
		Data: &workflow.WorkFlowTestRunData{
			WorkflowID: req.WorkflowID,
			ExecuteID:  fmt.Sprintf("%d", exeID),
		},
	}, nil
}

// GetCanvasInfoByWanwu 参考GetCanvasInfo
func (w *ApplicationService) GetCanvasInfoByWanwu(ctx context.Context, req *ExportWorkflowRequest) (
	_ *workflow.GetCanvasInfoResponse, err error,
) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			err = safego.NewPanicErr(panicErr, debug.Stack())
		}
		if err != nil {
			err = vo.WrapIfNeeded(errno.ErrWorkflowOperationFail, err, errorx.KV("cause", vo.UnwrapRootErr(err).Error()))
		}
	}()
	switch req.QType {
	case workflowModel.FromDraft:
		return w.GetCanvasInfo(ctx, &workflow.GetCanvasInfoRequest{
			SpaceID:    req.SpaceID,
			WorkflowID: ptr.Of(req.WorkflowID),
		})
	case workflowModel.FromLatestVersion, workflowModel.FromSpecificVersion:
		// do nothing, continue
	default:
		return nil, fmt.Errorf("invalid type")
	}

	wf, err := GetWorkflowDomainSVC().Get(ctx, &vo.GetPolicy{
		ID:      mustParseInt64(req.WorkflowID),
		QType:   req.QType,
		Version: req.Version,
	})
	if err != nil {
		return nil, err
	}
	var devStatus workflow.WorkFlowDevStatus
	var vcsType workflow.VCSCanvasType
	pluginID := "0"
	updateTime := time.Time{}

	canvasData := &workflow.CanvasData{
		Workflow: &workflow.Workflow{
			WorkflowID:       strconv.FormatInt(wf.ID, 10),
			Name:             wf.Name,
			Desc:             wf.Desc,
			URL:              wf.IconURL,
			IconURI:          wf.IconURI,
			Status:           devStatus,
			Type:             wf.ContentType,
			CreateTime:       wf.CreatedAt.Unix(),
			UpdateTime:       updateTime.Unix(),
			Tag:              wf.Tag,
			TemplateAuthorID: ternary.IFElse(wf.AuthorID > 0, ptr.Of(strconv.FormatInt(wf.AuthorID, 10)), nil),
			SpaceID:          ptr.Of(strconv.FormatInt(wf.SpaceID, 10)),
			SchemaJSON:       ptr.Of(wf.Canvas),
			Creator: &workflow.Creator{
				ID:   strconv.FormatInt(wf.CreatorID, 10),
				Self: ternary.IFElse[bool](wf.CreatorID == ptr.From(ctxutil.GetUIDFromCtx(ctx)), true, false),
			},
			FlowMode:         wf.Mode,
			ProjectID:        i64PtrToStringPtr(wf.AppID),
			PersistenceModel: workflow.PersistenceModel_VCS, // the front-end validation logic, this field returns VCS, developers don't need to pay attention
			PluginID:         pluginID,
		},
		VcsData: &workflow.VCSCanvasData{
			CanEdit:        true,
			SubmitCommitID: wf.CommitID,
			DraftCommitID:  wf.CommitID,
			Type:           vcsType,
		},
		WorkflowVersion: wf.LatestPublishedVersion,
	}

	return &workflow.GetCanvasInfoResponse{
		Data: canvasData,
	}, nil
}

// -- internal ---
type cozeWorkflowSelectByWanwuResp struct {
	Code int64 `json:"code"`
	Data struct {
		List []appId `json:"list"`
	} `json:"data"`
	Msg string `json:"msg"`
}

type appId struct {
	AppId string `json:"appId"` // 应用id
}

func patchWANWUHTTPAuthParamTypeInSchemaString(schemaStr string) (string, error) {
	if schemaStr == "" {
		return schemaStr, nil
	}
	var schemaMap map[string]any
	if err := sonic.Unmarshal([]byte(schemaStr), &schemaMap); err != nil {
		return "", err
	}
	patchWANWUHTTPAuthParamTypeInNodes(schemaMap["nodes"])
	patched, err := sonic.MarshalString(schemaMap)
	if err != nil {
		return "", err
	}
	return patched, nil
}

func patchWANWUHTTPAuthParamTypeInNodes(nodesAny any) {
	nodes, ok := nodesAny.([]any)
	if !ok {
		return
	}
	for _, nodeAny := range nodes {
		node, ok := nodeAny.(map[string]any)
		if !ok {
			continue
		}
		patchWANWUHTTPAuthParamTypeInSingleNode(node)
		patchWANWUHTTPAuthParamTypeInNodes(node["blocks"])
	}
}

func patchWANWUHTTPAuthParamTypeInSingleNode(node map[string]any) {
	nodeType, _ := node["type"].(string)
	if nodeType != "45" {
		return
	}
	data, ok := node["data"].(map[string]any)
	if !ok {
		return
	}
	inputs, ok := data["inputs"].(map[string]any)
	if !ok {
		return
	}
	auth, ok := inputs["auth"].(map[string]any)
	if !ok {
		return
	}
	authData, ok := auth["authData"].(map[string]any)
	if !ok {
		return
	}
	patchWANWUHTTPAuthParamTypeInList(authData, "bearerTokenData")
	patchWANWUHTTPAuthParamTypeInList(authData, "basicAuthData")
	if customData, ok := authData["customData"].(map[string]any); ok {
		patchWANWUHTTPAuthParamTypeInList(customData, "data")
	}
}

func patchWANWUHTTPAuthParamTypeInList(container map[string]any, listKey string) {
	items, ok := container[listKey].([]any)
	if !ok {
		return
	}
	for _, itemAny := range items {
		param, ok := itemAny.(map[string]any)
		if !ok {
			continue
		}
		input, ok := param["input"].(map[string]any)
		if !ok {
			continue
		}
		if inputType, ok := input["type"]; ok {
			param["type"] = inputType
		}
	}
}

type GetWorkflowRequest struct {
	WorkflowID string `thrift:"workflow_id,1,required" form:"workflow_id,required" json:"workflow_id,required" query:"workflow_id,required"`
}

type RunWorkFlowLatestVersionByWanwuReq struct {
	WorkflowID string         `json:"workflow_id,required" validate:"required"`
	Input      map[string]any `json:"input,omitempty"`
}

type ExportWorkflowRequest struct {
	WorkflowID string        `thrift:"workflow_id,1,required" form:"workflow_id,required" json:"workflow_id,required" query:"workflow_id,required"`
	SpaceID    string        `form:"space_id,required" json:"space_id" query:"space_id,required"`
	Version    string        `thrift:"version,6,optional" form:"version" json:"version,omitempty" query:"version"`
	QType      model.Locator `form:"qType" json:"qType" query:"qType"`
}

type CopyWorkflowRequest struct {
	WorkflowID string        `thrift:"workflow_id,1,required" form:"workflow_id,required" json:"workflow_id,required" query:"workflow_id,required"`
	SpaceID    string        `thrift:"space_id,2,required" form:"space_id,required" json:"space_id,required" query:"space_id,required"`
	SchemaJSON *string       `thrift:"schema_json,19,optional" form:"schema_json" json:"schema_json,omitempty" query:"schema_json"`
	QType      model.Locator `form:"qType" json:"qType" query:"qType"`
}
