package workflow

import (
	"context"
	"fmt"
	"runtime/debug"
	"strconv"
	"sync"

	"github.com/coze-dev/coze-studio/backend/api/model/workflow"
	"github.com/coze-dev/coze-studio/backend/application/base/ctxutil"
	crossconversation "github.com/coze-dev/coze-studio/backend/crossdomain/conversation"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/entity/vo"
	"github.com/coze-dev/coze-studio/backend/pkg/errorx"
	"github.com/coze-dev/coze-studio/backend/pkg/lang/maps"
	"github.com/coze-dev/coze-studio/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-studio/backend/pkg/lang/ternary"
	"github.com/coze-dev/coze-studio/backend/pkg/safego"
	"github.com/coze-dev/coze-studio/backend/types/errno"
)

// CreateApplicationConversationDefByWanwu 参考CreateApplicationConversationDef
func (w *ApplicationService) CreateApplicationConversationDefByWanwu(ctx context.Context, req *workflow.CreateProjectConversationDefRequest) (resp *workflow.CreateProjectConversationDefResponse, err error) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			err = safego.NewPanicErr(panicErr, debug.Stack())
		}

		if err != nil {
			err = vo.WrapIfNeeded(errno.ErrConversationOfAppOperationFail, err, errorx.KV("cause", vo.UnwrapRootErr(err).Error()))
		}
	}()
	// 取消获取appID，返回随机生成的newAppId（放在uniqueID字段中）作为app-service的applicationId
	var (
		spaceID = mustParseInt64(req.GetSpaceID())
		//appID   = mustParseInt64(req.GetProjectID())
		userID = ctxutil.MustGetUIDFromCtx(ctx)
	)

	if err := checkUserSpace(ctx, userID, spaceID); err != nil {
		return nil, err
	}
	newAppID, _ := w.IDGenerator.GenID(ctx)
	// 应用广场默认创建conversation template，表现为用户首次进入应用广场对话流，会有一个默认的conversation
	_, err = GetWorkflowDomainSVC().CreateDraftConversationTemplate(ctx, &vo.CreateConversationTemplateMeta{
		AppID:   newAppID,
		SpaceID: spaceID,
		Name:    req.GetConversationName(),
		UserID:  userID,
	})
	if err != nil {
		return nil, err
	}

	return &workflow.CreateProjectConversationDefResponse{
		UniqueID: strconv.FormatInt(newAppID, 10),
		SpaceID:  req.GetSpaceID(),
	}, err
}

// OpenAPICreateConversationByWanwu 参考OpenAPICreateConversation
func (w *ApplicationService) OpenAPICreateConversationByWanwu(ctx context.Context, req *workflow.CreateConversationRequest) (resp *workflow.CreateConversationResponse, err error) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			err = safego.NewPanicErr(panicErr, debug.Stack())
		}
		if err != nil {
			err = vo.WrapIfNeeded(errno.ErrWorkflowOperationFail, err, errorx.KV("cause", vo.UnwrapRootErr(err).Error()))
		}
	}()

	var (
		appID      = mustParseInt64(req.GetAppID())
		apiKeyInfo = ctxutil.GetApiAuthFromCtx(ctx)
		userID     = apiKeyInfo.UserID
		env        = ternary.IFElse(req.GetDraftMode(), vo.Draft, vo.Online)
		cID        int64
	)

	// 检查 workflowID 是否存在
	if req.WorkflowID != nil && *req.WorkflowID != "" {
		workflowIDInt, parseErr := strconv.ParseInt(*req.WorkflowID, 10, 64)
		if parseErr != nil {
			return nil, vo.WrapError(errno.ErrInvalidParameter, fmt.Errorf("invalid workflow_id: %w", parseErr),
				errorx.KV("workflow_id", *req.WorkflowID))
		}
		_, err = GetWorkflowDomainSVC().Get(ctx, &vo.GetPolicy{
			ID:       workflowIDInt,
			MetaOnly: true,
		})
		if err != nil {
			return nil, err
		}
	}

	// todo  check permission

	if !req.GetGetOrCreate() {
		cID, err = GetWorkflowDomainSVC().UpdateConversation(ctx, env, appID, req.GetConnectorId(), userID, req.GetConversationMame())
	} else {
		var tplExisted, dcExisted bool
		var tplErr, dcErr error
		var wg sync.WaitGroup
		wg.Add(2)

		safego.Go(ctx, func() {
			defer wg.Done()
			_, tplExisted, tplErr = GetWorkflowDomainSVC().GetTemplateByName(ctx, env, appID, req.GetConversationMame())
		})

		safego.Go(ctx, func() {
			defer wg.Done()
			_, dcExisted, dcErr = GetWorkflowDomainSVC().GetDynamicConversationByName(ctx, env, appID, req.GetConnectorId(), userID, req.GetConversationMame())
		})

		wg.Wait()

		if tplErr != nil {
			return nil, tplErr
		}
		if dcErr != nil {
			return nil, dcErr
		}

		if !tplExisted && !dcExisted {
			// 去除coze原始代码检查数据是否存在，一定创建conversation
		}

		cID, _, err = GetWorkflowDomainSVC().GetOrCreateConversation(ctx, env, appID, req.GetConnectorId(), userID, req.GetConversationMame())

	}
	if err != nil {
		return nil, err
	}

	cInfo, err := crossconversation.DefaultSVC().GetByID(ctx, cID)
	if err != nil {
		return nil, err
	}

	return &workflow.CreateConversationResponse{
		ConversationData: &workflow.ConversationData{
			Id:            cID,
			LastSectionID: ptr.Of(cInfo.SectionID),
		},
	}, nil
}

// DeleteApplicationConversationDef 参考DeleteApplicationConversationDef
func (w *ApplicationService) DeleteApplicationConversationDefByWanwu(ctx context.Context, req *workflow.DeleteProjectConversationDefRequest) (resp *workflow.DeleteProjectConversationDefResponse, err error) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			err = safego.NewPanicErr(panicErr, debug.Stack())
		}

		if err != nil {
			err = vo.WrapIfNeeded(errno.ErrConversationOfAppOperationFail, err, errorx.KV("cause", vo.UnwrapRootErr(err).Error()))
		}
	}()
	var (
		appID      = mustParseInt64(req.GetProjectID())
		templateID = mustParseInt64(req.GetUniqueID())
	)
	if err := checkUserSpace(ctx, ctxutil.MustGetUIDFromCtx(ctx), mustParseInt64(req.GetSpaceID())); err != nil {
		return nil, err
	}
	if req.GetCheckOnly() {
		wfs, err := GetWorkflowDomainSVC().CheckWorkflowsToReplace(ctx, appID, templateID)
		if err != nil {
			return nil, err
		}
		resp = &workflow.DeleteProjectConversationDefResponse{NeedReplace: make([]*workflow.Workflow, 0)}
		for _, wf := range wfs {
			resp.NeedReplace = append(resp.NeedReplace, &workflow.Workflow{
				Name:       wf.Name,
				URL:        wf.IconURL,
				WorkflowID: strconv.FormatInt(wf.ID, 10),
			})
		}
		return resp, nil
	}

	wfID2ConversationName, err := maps.TransformKeyWithErrorCheck(req.GetReplace(), func(k1 string) (int64, error) {
		return strconv.ParseInt(k1, 10, 64)
	})

	rowsAffected, err := GetWorkflowDomainSVC().DeleteDraftConversationTemplate(ctx, templateID, wfID2ConversationName)
	if err != nil {
		return nil, err
	}
	if rowsAffected > 0 {
		return &workflow.DeleteProjectConversationDefResponse{
			Success: true,
		}, err
	}
	// vo.draft->vo.online 用于前端删除应用广场创建的对话
	rowsAffected, err = GetWorkflowDomainSVC().DeleteDynamicConversation(ctx, vo.Online, templateID)
	if err != nil {
		return nil, err
	}

	if rowsAffected == 0 {
		return nil, fmt.Errorf("delete conversation failed")
	}

	return &workflow.DeleteProjectConversationDefResponse{
		Success: true,
	}, nil

}
