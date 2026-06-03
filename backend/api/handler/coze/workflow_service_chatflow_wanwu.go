package coze

import (
	"context"
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
// 0. FIXME 前端运行该接口，不会在header中带userId、orgId，需要在该方法中设置ctxcache
// @router /v1/workflows/chat [POST]
func OpenAPIChatFlowRunByWanwu(ctx context.Context, c *app.RequestContext) {
	var err error
	if err = preprocessWorkflowRequestBody(ctx, c); err != nil {
		invalidParamRequestResponse(c, err.Error())
		return
	}

	var req workflow.ChatFlowRunRequest
	var userId, orgId string
	var startTime time.Time
	err = c.BindAndValidate(&req)
	if err != nil {
		invalidParamRequestResponse(c, err.Error())
		return
	}

	// 设置ctxcache
	if ctxutil.GetUserSessionFromCtx(ctx) == nil {
		meta, err := appworkflow.GetWorkflowDomainSVC().Get(ctx, &vo.GetPolicy{
			ID:       mustParseInt64(req.WorkflowID),
			MetaOnly: true,
		})
		if err != nil {
			internalServerErrorResponse(ctx, c, err)
			return
		}
		ctxcache.Store(ctx, tConsts.SessionDataKeyInCtx, &user_entity.Session{
			UserID: meta.CreatorID,
			Locale: string(i18n.GetLocale(ctx)),
		})
		ctxcache.Store(ctx, "X-Org-Id", strconv.Itoa(int(meta.SpaceID)))
		userId, orgId = strconv.Itoa(int(meta.CreatorID)), strconv.Itoa(int(meta.SpaceID))
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
	startTime = time.Now()
	sr, err := appworkflow.SVC.OpenAPIChatFlowRun(ctx, &req)
	if err != nil {
		// 必须满足以下条件才调用回调接口：（只给应用广场调用记录）
		// 1. 执行模式不为 DEBUG
		// 2. openAPI调用的时候userId 和 orgId 为空，openAPI调用已经在bff记录了，不加userId orgId判断会导致重复记录
		if req.GetExecuteMode() != "DEBUG" && userId != "" && orgId != "" {
			callWanwuRecordAppAPI(ctx, userId, orgId, req.WorkflowID, false, 0, 0)
		}
		internalServerErrorResponse(ctx, c, err)
		return
	}
	// sendChatFlowStreamRunSSEByWanwu 返回首token时延和总流时延
	ttft, totalCosts := sendChatFlowStreamRunSSEByWanwu(ctx, w, sr, startTime)

	if req.GetExecuteMode() != "DEBUG" && userId != "" && orgId != "" {
		callWanwuRecordAppAPI(ctx, userId, orgId, req.WorkflowID, true, totalCosts, ttft)
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

// callWanwuRecordAppAPI 调用万物回调接口获取已发布的工作流列表
// userID: 用户ID
// orgID: 组织ID（空间ID）
// appID: 应用ID
// ttft: 首token时延(ms)
// streamCosts: 总流时延(ms)
// 来源为应用广场对话流的调用
func callWanwuRecordAppAPI(ctx context.Context, userId, orgId, appId string, isSuccess bool, streamCosts, ttft int64) {
	_, err := resty.New().
		R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBody(map[string]any{
			"userId":      userId,
			"orgId":       orgId,
			"appId":       appId,
			"isSuccess":   isSuccess,
			"streamCosts": streamCosts,
			"ttft":        ttft,
			"source":      "web",
			"appType":     "chatflow",
			"isStream":    true,
		}).
		Post(os.Getenv("WANWU_CALLBACK_APP_RECORD_URL"))

	if err != nil {
		logs.Info("-----------------------------------------------------------------------")
		logs.Errorf("callWanwuCallbackAPI error, userID: %s, orgID: %s, appID: %s,error:%v", userId, orgId, appId, err)
		logs.Info("-----------------------------------------------------------------------")
		return
	}
	logs.Info("-----------------------------------------------------------------------")
	logs.Infof("callWanwuCallbackAPI completed, userID: %s, orgID: %s, appID: %s", userId, orgId, appId)
	logs.Info("-----------------------------------------------------------------------")
}

// sendChatFlowStreamRunSSEByWanwu 发送流式SSE事件并返回首token时延和总流时延
func sendChatFlowStreamRunSSEByWanwu(ctx context.Context, w *sse.Writer, sr *schema.StreamReader[[]*workflow.ChatFlowRunResponse], startTime time.Time) (ttft int64, totalCosts int64) {
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
	return ttft, totalCosts
}
