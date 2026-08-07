package coze

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/hertz/pkg/protocol/sse"
	"github.com/coze-dev/coze-studio/backend/api/model/workflow"
	"github.com/coze-dev/coze-studio/backend/application/base/ctxutil"
	appworkflow "github.com/coze-dev/coze-studio/backend/application/workflow"
	user_entity "github.com/coze-dev/coze-studio/backend/domain/user/entity"
	wfentity "github.com/coze-dev/coze-studio/backend/domain/workflow/entity"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/entity/vo"
	"github.com/coze-dev/coze-studio/backend/pkg/ctxcache"
	"github.com/coze-dev/coze-studio/backend/pkg/i18n"
	"github.com/coze-dev/coze-studio/backend/pkg/logs"
	tConsts "github.com/coze-dev/coze-studio/backend/types/consts"
	"github.com/go-resty/resty/v2"
)

// OpenAPIGetWorkflowInfoByWanwu 参考OpenAPIGetWorkflowInfo
// 0. FIXME 前端运行该接口，不会在header中带orgId，需要在该方法中设置ctxcache
// @router /v1/workflows/:workflow_id [GET]
func OpenAPIGetWorkflowInfoByWanwu(ctx context.Context, c *app.RequestContext) {
	var err error

	if err = processOpenAPIGetWorkflowInfoRequest(ctx, c); err != nil {
		invalidParamRequestResponse(c, err.Error())
		return
	}

	var req workflow.OpenAPIGetWorkflowInfoRequest

	err = c.BindAndValidate(&req)
	if err != nil {
		invalidParamRequestResponse(c, err.Error())
		return
	}

	// 设置ctxcache
	wf, err := appworkflow.GetWorkflowDomainSVC().Get(ctx, &vo.GetPolicy{
		ID:       mustParseInt64(req.GetWorkflowID()),
		MetaOnly: true,
	})
	if err != nil {
		internalServerErrorResponse(ctx, c, err)
		return
	}
	if _, ok := ctxcache.Get[string](ctx, "X-Org-Id"); !ok {
		ctxcache.Store(ctx, "X-Org-Id", strconv.Itoa(int(wf.Meta.SpaceID)))
	}

	resp, err := appworkflow.SVC.OpenAPIGetWorkflowInfo(ctx, &req)
	if err != nil {
		internalServerErrorResponse(ctx, c, err)
		return
	}

	c.JSON(consts.StatusOK, resp)
}

// OpenAPIChatFlowRunByWanwu 参考OpenAPIChatFlowRun
// 应用统计谁记：
//   - 广场对话流：FE 直连本接口（pat_、无 Session）→ 本接口记
//   - 草稿 DEBUG：有 Session/X-User-Id → 本接口记
//   - OpenAPI：BFF→chat_by_wanwu 带 X-User-Id 建 Session → BFF 记，本接口跳过
//
// @router /v1/workflows/chat [POST]
// @router /v1/workflow/chat_by_wanwu [POST]
func OpenAPIChatFlowRunByWanwu(ctx context.Context, c *app.RequestContext) {
	var err error
	if err = preprocessWorkflowRequestBody(ctx, c); err != nil {
		invalidParamRequestResponse(c, err.Error())
		return
	}

	var req workflow.ChatFlowRunRequest
	err = c.BindAndValidate(&req)
	if err != nil {
		invalidParamRequestResponse(c, err.Error())
		return
	}

	userId, orgId, hadSession, err := resolveChatflowRecordCaller(ctx, c, req.WorkflowID)
	if err != nil {
		internalServerErrorResponse(ctx, c, err)
		return
	}

	w := sse.NewWriter(c)
	c.SetContentType("text/event-stream; charset=utf-8")
	c.Response.Header.Set("Cache-Control", "no-cache")
	c.Response.Header.Set("Connection", "keep-alive")
	c.Response.Header.Set("Access-Control-Allow-Origin", "*")
	//处理 parameters 字段，防止 JSON "null" 导致 map 变成 nil
	if req.Parameters == nil || *req.Parameters == "" || *req.Parameters == "null" {
		emptyJSON := "{}"
		req.Parameters = &emptyJSON
	}
	startTime := time.Now()
	isDebug := req.GetExecuteMode() == "DEBUG"
	// DEBUG / 广场(!hadSession) 本接口记；BFF OpenAPI(hadSession && !DEBUG) 跳过防双记
	shouldRecord := userId != "" && orgId != "" && (isDebug || !hadSession)
	// 应用统计 requestBody 落整体 HTTP 请求（ChatFlowRunRequest）；question 取最后一条 user 消息。
	reqBody := ""
	if b, e := json.Marshal(&req); e == nil {
		reqBody = string(b)
	}
	question := chatflowUserMessage(req.GetAdditionalMessages())
	sr, err := appworkflow.SVC.OpenAPIChatFlowRun(ctx, &req)
	if err != nil {
		if shouldRecord {
			callWanwuRecordAppAPI(ctx, userId, orgId, req.WorkflowID, "chatflow", true, false, 0, reqBody, "", question, "", err.Error())
		}
		internalServerErrorResponse(ctx, c, err)
		return
	}
	ttft, _, streamErr := sendChatFlowStreamRunSSEByWanwu(ctx, w, sr, startTime)

	if shouldRecord {
		if streamErr != nil {
			callWanwuRecordAppAPI(ctx, userId, orgId, req.WorkflowID, "chatflow", true, false, ttft, reqBody, "", question, "", streamErr.Error())
		} else {
			callWanwuRecordAppAPI(ctx, userId, orgId, req.WorkflowID, "chatflow", true, true, ttft, reqBody, "", question, "", "")
		}
	}
}

// CreateProjectConversationDefByWanwu 参考CreateProjectConversationDef
// @router /api/workflow_api/project_conversation/create_by_wanwu [POST]
func CreateProjectConversationDefByWanwu(ctx context.Context, c *app.RequestContext) {
	var err error
	var req workflow.CreateProjectConversationDefRequest
	err = c.BindAndValidate(&req)
	if err != nil {
		invalidParamRequestResponse(c, err.Error())
		return
	}

	resp, err := appworkflow.SVC.CreateApplicationConversationDefByWanwu(ctx, &req)
	if err != nil {
		internalServerErrorResponse(ctx, c, err)
		return
	}

	c.JSON(consts.StatusOK, resp)
}

// wanwuAppRecordClient 复用 HTTP 客户端，避免每次回调 resty.New()。
var wanwuAppRecordClient = resty.New()

// recordDraftTestRunAfterFinish 后台等到执行终态，用 time.Since(startTime) 记 nonStreamCosts。
// 脱离 HTTP cancel，保留原 TraceID，与对话流「自己掐时间」一致。
// question 为试运行入参 JSON；成功时 answer=responseBody（与 BFF 工作流约定一致）。
func recordDraftTestRunAfterFinish(ctx context.Context, userId, orgId, workflowID, executeID, appType, reqBody, question string, startTime time.Time) {
	detached := context.WithoutCancel(ctx)
	go func() {
		ok, failReason, respBody := waitDraftWorkflowFinish(detached, workflowID, executeID)
		costs := time.Since(startTime).Milliseconds()
		reason := ""
		if !ok {
			reason = failReason
		}
		answer := ""
		if ok {
			answer = respBody
		}
		callWanwuRecordAppAPI(detached, userId, orgId, workflowID, appType, false, ok, costs, reqBody, respBody, question, answer, reason)
	}()
}

// waitDraftWorkflowFinish 轮询 GetExecution，直到 Success/Fail/Cancel 或超时；
// 成功时带回 exe.Output 作为统计 responseBody。
func waitDraftWorkflowFinish(ctx context.Context, workflowID, executeID string) (ok bool, failReason, responseBody string) {
	wfID, err := strconv.ParseInt(workflowID, 10, 64)
	if err != nil {
		return false, "invalid workflowID", ""
	}
	exeID, err := strconv.ParseInt(executeID, 10, 64)
	if err != nil {
		return false, "invalid executeID", ""
	}
	pollCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-pollCtx.Done():
			logs.CtxWarnf(ctx, "waitDraftWorkflowFinish timeout workflow=%s execute=%s", workflowID, executeID)
			return false, "workflow execution timeout", ""
		case <-ticker.C:
			exe, err := appworkflow.GetWorkflowDomainSVC().GetExecution(pollCtx, &wfentity.WorkflowExecution{
				ID:         exeID,
				WorkflowID: wfID,
			}, false)
			if err != nil {
				logs.CtxWarnf(ctx, "waitDraftWorkflowFinish get execution err workflow=%s execute=%s: %v",
					workflowID, executeID, err)
				continue
			}
			switch exe.Status {
			case wfentity.WorkflowSuccess:
				if exe.Output != nil {
					responseBody = *exe.Output
				}
				return true, "", responseBody
			case wfentity.WorkflowFailed:
				reason := "workflow execution failed"
				if exe.FailReason != nil && *exe.FailReason != "" {
					reason = *exe.FailReason
				}
				return false, reason, ""
			case wfentity.WorkflowCancel:
				return false, "workflow execution canceled", ""
			}
		}
	}
}

// callWanwuRecordAppAPI 回调 BFF /callback/v1/app/record。
// isStream=true 时 costs 为 TTFT(ms)；false 时为 nonStreamCosts（自己掐的 wall-clock ms）。
// failureReason 在 isSuccess=false 时透传给 BFF 作为统计 failureReason；成功时传空。
// statusCode 由 isSuccess 推导（成功 200 / 失败 500），BFF 直接透传到统计记录。
// responseBody：非流式成功时传 exe.Output；流式传空。
// question/answer：精简问答摘要；非流式工作流 answer 通常等于 responseBody；流式 answer 可空。
func callWanwuRecordAppAPI(ctx context.Context, userId, orgId, appId, appType string, isStream, isSuccess bool, costs int64, requestBody, responseBody, question, answer, failureReason string) {
	statusCode := int64(200)
	if !isSuccess {
		statusCode = 500
	}
	body := map[string]any{
		"userId":        userId,
		"orgId":         orgId,
		"appId":         appId,
		"statusCode":    statusCode,
		"failureReason": failureReason,
		"source":        "web",
		"appType":       appType,
		"module":        "workflow",
		"isStream":      isStream,
		"requestBody":   requestBody,
		"responseBody":  responseBody,
		"question":      question,
		"answer":        answer,
	}
	if isStream {
		body["streamCosts"] = costs
	} else {
		body["nonStreamCosts"] = costs
	}
	_, err := wanwuAppRecordClient.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBody(body).
		Post(os.Getenv("WANWU_CALLBACK_APP_RECORD_URL"))
	if err != nil {
		logs.CtxErrorf(ctx, "callWanwuRecordAppAPI error userID=%s orgID=%s appID=%s appType=%s: %v",
			userId, orgId, appId, appType, err)
		return
	}
	logs.CtxInfof(ctx, "callWanwuRecordAppAPI ok userID=%s orgID=%s appID=%s appType=%s stream=%v",
		userId, orgId, appId, appType, isStream)
}

// chatflowUserMessage 取 additional_messages 中最后一条 user 文本，供对话流统计 question 字段。
func chatflowUserMessage(messages []*workflow.EnterMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i] == nil {
			continue
		}
		if messages[i].GetRole() == "user" && messages[i].GetContent() != "" {
			return messages[i].GetContent()
		}
	}
	return ""
}

// resolveChatflowRecordCaller 解析对话流统计调用人，并在无 Session 时用草稿 meta 补齐（广场 pat_）。
func resolveChatflowRecordCaller(ctx context.Context, c *app.RequestContext, workflowID string) (userID, orgID string, hadSession bool, err error) {
	hadSession = ctxutil.GetUserSessionFromCtx(ctx) != nil
	userID, orgID = resolveWorkflowDraftRecordUser(ctx, c, "")
	if hadSession {
		return userID, orgID, true, nil
	}
	meta, err := appworkflow.GetWorkflowDomainSVC().Get(ctx, &vo.GetPolicy{
		ID:       mustParseInt64(workflowID),
		MetaOnly: true,
	})
	if err != nil {
		return "", "", false, err
	}
	ctxcache.Store(ctx, tConsts.SessionDataKeyInCtx, &user_entity.Session{
		UserID: meta.CreatorID,
		Locale: string(i18n.GetLocale(ctx)),
	})
	orgID = strconv.FormatInt(meta.SpaceID, 10)
	ctxcache.Store(ctx, "X-Org-Id", orgID)
	userID = strconv.FormatInt(meta.CreatorID, 10)
	return userID, orgID, false, nil
}

// resolveWorkflowDraftRecordUser 解析调用人：header → ctxcache org → body space_id → session。
// SetUserID 只写 Session，不会把 "X-User-Id" 字符串写入 ctxcache。
func resolveWorkflowDraftRecordUser(ctx context.Context, c *app.RequestContext, bodySpaceID string) (userID, orgID string) {
	userID = string(c.GetHeader("X-User-Id"))
	if userID == "" {
		userID = string(c.GetHeader("x-user-id"))
	}
	orgID = string(c.GetHeader("X-Org-Id"))
	if orgID == "" {
		orgID = string(c.GetHeader("x-org-id"))
	}
	if orgID == "" {
		if v, ok := ctxcache.Get[string](ctx, "X-Org-Id"); ok {
			orgID = v
		}
	}
	if orgID == "" && bodySpaceID != "" {
		orgID = bodySpaceID
	}
	if userID == "" {
		if session := ctxutil.GetUserSessionFromCtx(ctx); session != nil {
			userID = strconv.FormatInt(session.UserID, 10)
		}
	}
	return userID, orgID
}

// resolveWorkflowDraftAppType 根据 workflow 元信息判断 appType（workflow / chatflow）。
func resolveWorkflowDraftAppType(ctx context.Context, workflowID string) string {
	wfID, _ := strconv.ParseInt(workflowID, 10, 64)
	wf, err := appworkflow.GetWorkflowDomainSVC().Get(ctx, &vo.GetPolicy{
		ID:       wfID,
		MetaOnly: true,
	})
	if err != nil || wf == nil || wf.Meta == nil {
		return "workflow"
	}
	if wf.Mode == workflow.WorkflowMode_ChatFlow {
		return "chatflow"
	}
	return "workflow"
}

// sendChatFlowStreamRunSSEByWanwu 发送流式SSE事件并返回首token时延和总流时延
func sendChatFlowStreamRunSSEByWanwu(ctx context.Context, w *sse.Writer, sr *schema.StreamReader[[]*workflow.ChatFlowRunResponse], startTime time.Time) (ttft int64, totalCosts int64, streamErr error) {
	defer func() {
		_ = w.Close()
		sr.Close()
		totalCosts = int64(time.Since(startTime).Milliseconds())
	}()

	seq := int64(1)
	firstTokenReceived := false
	for {
		respList, err := sr.Recv()

		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			streamErr = err
			event := &sse.Event{
				Type: "error",
				Data: []byte(err.Error()),
			}

			if err = w.Write(event); err != nil {
				logs.CtxErrorf(ctx, "publish stream event failed, err:%v", err)
			}
			return
		}

		if !firstTokenReceived {
			ttft = int64(time.Since(startTime).Milliseconds())
			firstTokenReceived = true
		}

		for _, resp := range respList {
			event := &sse.Event{
				ID:   strconv.FormatInt(seq, 10),
				Type: resp.Event,
				Data: []byte(resp.Data),
			}

			if err = w.Write(event); err != nil {
				logs.CtxErrorf(ctx, "publish stream event failed, err:%v", err)
				return
			}
			seq++
		}

	}
	return ttft, totalCosts, nil
}
