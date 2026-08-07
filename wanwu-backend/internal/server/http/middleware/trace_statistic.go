package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/coze-dev/coze-studio/backend/api/model/workflow"
	appworkflow "github.com/coze-dev/coze-studio/backend/application/workflow"
	workflowModel "github.com/coze-dev/coze-studio/backend/crossdomain/workflow/model"
	user_entity "github.com/coze-dev/coze-studio/backend/domain/user/entity"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/entity/vo"
	"github.com/coze-dev/coze-studio/backend/pkg/ctxcache"
	"github.com/coze-dev/coze-studio/backend/pkg/logs"
	"github.com/coze-dev/coze-studio/backend/pkg/sonic"
	trace_util "github.com/coze-dev/coze-studio/backend/pkg/trace-util"
	"github.com/coze-dev/coze-studio/backend/types/consts"

	"github.com/UnicomAI/wanwu-workflow/wanwu-backend/config"
	"github.com/UnicomAI/wanwu-workflow/wanwu-backend/pkg/statistictrace"
)

// TraceStatisticGlobal 在 tracing 之后按路径写入模型统计 trace，避免路由级中间件拿不到 traceId。
// 对话流：/v1/workflows/chat、/v1/workflow/chat_by_wanwu；试运行：/api/workflow_api/test_run 等。
func TraceStatisticGlobal() app.HandlerFunc {
	chatflowPaths := map[string]bool{
		"/v1/workflows/chat":         true,
		"/v1/workflow/chat_by_wanwu": true,
	}
	draftRunPaths := map[string]bool{
		"/api/workflow_api/test_run":    true,
		"/api/workflow_api/test_resume": true,
		"/api/workflow_api/nodeDebug":   true,
	}
	return func(ctx context.Context, c *app.RequestContext) {
		path := string(c.Path())
		switch {
		case chatflowPaths[path]:
			persistChatflowRunTrace(ctx, c)
		case draftRunPaths[path]:
			persistWorkflowDraftRunTrace(ctx, c)
		}
		// 试运行/对话流异步执行：脱离 HTTP request cancel，保留 OTEL TraceID，供模型 callback 注入。
		if chatflowPaths[path] || draftRunPaths[path] {
			c.Next(context.WithoutCancel(ctx))
			return
		}
		c.Next(ctx)
	}
}

type workflowTraceBody struct {
	WorkflowID  json.Number `json:"workflow_id"`
	SpaceID     json.Number `json:"space_id"`
	ExecuteMode string      `json:"execute_mode"`
}

func persistWorkflowDraftRunTrace(ctx context.Context, c *app.RequestContext) {
	persistWorkflowTrace(ctx, c, false, "")
}

func persistChatflowRunTrace(ctx context.Context, c *app.RequestContext) {
	persistWorkflowTrace(ctx, c, true, statistictrace.AppTypeChatflow)
}

// persistWorkflowTrace 从请求解析 workflow 信息，写入 Redis trace_user:{traceId}，供 BFF 模型统计读取。
func persistWorkflowTrace(ctx context.Context, c *app.RequestContext, chatflow bool, defaultAppType string) {
	if statistictrace.OP() == nil {
		logs.CtxWarnf(ctx, "trace statistic run skip: statistic redis OP not initialized")
		return
	}

	// 须在 hertztracing 之后执行，span 未就绪时不写。
	if !trace_util.IsTraceContextValid(ctx) {
		return
	}
	traceID := trace_util.GetTraceID(ctx)

	// BFF 或本服务已写入完整统计字段时跳过。
	if ok, _ := statistictrace.HasDraftRunTrace(ctx, traceID); ok {
		return
	}

	body := parseWorkflowTraceBody(ctx, c)
	workflowID := body.WorkflowID.String()
	if workflowID == "" {
		return
	}

	// 一次读取 workflow 草稿元信息（创建人 + app 类型），供下面补全各字段。
	meta, err := lookupWorkflowMeta(ctx, workflowID)
	if err != nil {
		logs.CtxWarnf(ctx, "trace statistic run lookup workflow %s meta err: %v", workflowID, err)
	}

	// 调用人：优先 header/session，缺失时回退到 workflow 创建人。
	callerUserID, callerOrgID := resolveTraceCaller(ctx, c, body.SpaceID.String())
	if callerUserID == "" {
		callerUserID = meta.creatorUserID
	}
	if callerOrgID == "" {
		callerOrgID = meta.creatorOrgID
	}
	if callerUserID == "" || callerOrgID == "" {
		logs.CtxWarnf(ctx, "trace statistic run skip: missing caller userId/orgId workflow=%s", workflowID)
		return
	}

	// app 类型：chatflow 入口已知；workflow 试运行用元信息推断，兜底为 workflow。
	appType := defaultAppType
	if appType == "" {
		appType = meta.appType
	}
	if appType == "" {
		appType = statistictrace.AppTypeWorkflow
	}

	// 创建人：试运行/DEBUG 取当前调用人；chatflow 正式发布取 workflow 创建人。
	creatorUserID, creatorOrgID := callerUserID, callerOrgID
	if chatflow && body.ExecuteMode != "DEBUG" && meta.creatorUserID != "" {
		creatorUserID, creatorOrgID = meta.creatorUserID, meta.creatorOrgID
	}

	if err := statistictrace.SaveDraftRunTrace(ctx, traceID, statistictrace.DraftRunTrace{
		APIPath:       string(c.Path()),
		WorkflowID:    workflowID,
		AppType:       appType,
		CallerUserID:  callerUserID,
		CallerOrgID:   callerOrgID,
		CreatorUserID: creatorUserID,
		CreatorOrgID:  creatorOrgID,
	}); err != nil {
		logs.CtxWarnf(ctx, "trace statistic run persist err: %v", err)
	}
}

func parseWorkflowTraceBody(ctx context.Context, c *app.RequestContext) workflowTraceBody {
	var body workflowTraceBody
	if len(c.Request.Body()) == 0 {
		return body
	}
	if err := sonic.Unmarshal(c.Request.Body(), &body); err != nil {
		logs.CtxWarnf(ctx, "trace statistic parse body err: %v", err)
	}
	return body
}

// resolveTraceCaller 解析统计所需的调用人 userId/orgId。
// orgId：header X-Org-ID 优先（SetOrgID 中间件亦从 header 写入 ctxcache，两者同源）；
//
//	header 缺失时用 body space_id 兜底（前端试运行接口 test_run/test_resume/nodeDebug
//	在 body 传 space_id）；再缺失则读 ctxcache。
//
// userId：header 优先，缺失时从 session（SetUserID 中间件写入）取。
func resolveTraceCaller(ctx context.Context, c *app.RequestContext, bodySpaceID string) (userID, orgID string) {
	userID = c.Request.Header.Get(config.X_USER_ID)
	orgID = c.Request.Header.Get(config.X_ORG_ID)
	if orgID == "" && bodySpaceID != "" {
		orgID = bodySpaceID
	}
	if orgID == "" {
		if v, ok := ctxcache.Get[string](ctx, config.X_ORG_ID); ok {
			orgID = v
		}
	}
	if userID == "" {
		if session, ok := ctxcache.Get[*user_entity.Session](ctx, consts.SessionDataKeyInCtx); ok && session != nil {
			userID = strconv.FormatInt(session.UserID, 10)
		}
	}
	return userID, orgID
}

// workflowMeta 是统计所需的 workflow 草稿元信息。
type workflowMeta struct {
	creatorUserID string
	creatorOrgID  string
	appType       string
}

// lookupWorkflowMeta 读取 workflow 草稿元信息：创建人 userId/orgId 及 app 类型（chatflow / workflow）。
func lookupWorkflowMeta(ctx context.Context, workflowID string) (workflowMeta, error) {
	wfID, err := strconv.ParseInt(workflowID, 10, 64)
	if err != nil {
		return workflowMeta{}, err
	}
	wf, err := appworkflow.GetWorkflowDomainSVC().Get(ctx, &vo.GetPolicy{
		ID:       wfID,
		QType:    workflowModel.FromDraft,
		MetaOnly: true,
	})
	if err != nil {
		return workflowMeta{}, err
	}
	if wf.Meta == nil {
		return workflowMeta{}, fmt.Errorf("workflow %s meta empty", workflowID)
	}
	appType := statistictrace.AppTypeWorkflow
	if wf.Mode == workflow.WorkflowMode_ChatFlow {
		appType = statistictrace.AppTypeChatflow
	}
	return workflowMeta{
		creatorUserID: strconv.FormatInt(wf.CreatorID, 10),
		creatorOrgID:  strconv.FormatInt(wf.SpaceID, 10),
		appType:       appType,
	}, nil
}
