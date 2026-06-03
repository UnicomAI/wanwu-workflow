package workflow

import (
	"context"
	"fmt"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/coze-dev/coze-studio/backend/api/model/base"
	"github.com/coze-dev/coze-studio/backend/api/model/workflow"
	workflowModel "github.com/coze-dev/coze-studio/backend/crossdomain/workflow/model"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/entity/vo"
	"github.com/coze-dev/coze-studio/backend/pkg/errorx"
	"github.com/coze-dev/coze-studio/backend/pkg/lang/ternary"
	"github.com/coze-dev/coze-studio/backend/pkg/safego"
	"github.com/coze-dev/coze-studio/backend/types/errno"
)

// GetWorkflowVersionListByWanwu 获取工作流的所有版本列表
func (w *ApplicationService) GetWorkflowVersionListByWanwu(ctx context.Context, workflowID string) (
	_ *GetWorkflowVersionListResponse, err error,
) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			err = safego.NewPanicErr(panicErr, debug.Stack())
		}
	}()
	versionInfos, err := GetWorkflowDomainSVC().GetWorkflowVersionListByWanwu(ctx, mustParseInt64(workflowID))
	if err != nil {
		return nil, err
	}
	response := GetWorkflowVersionListResponse{
		Data: &WorkflowVersionListData{
			WorkflowID:  workflowID,
			VersionList: make([]*WorkflowVersion, 0, len(versionInfos)),
			Total:       int32(len(versionInfos)),
		},
	}
	for _, versionInfo := range versionInfos {
		wv := &WorkflowVersion{
			Version:            versionInfo.Version,
			VersionDescription: versionInfo.VersionDescription,
			CreatedAt:          versionInfo.VersionCreatedAt.UnixMilli(),
			CommitID:           workflowID + "_" + versionInfo.Version,
			Type:               workflow.OperateType_PublishOperate,
		}
		response.Data.VersionList = append(response.Data.VersionList, wv)
	}

	return &response, nil
}

func (w *ApplicationService) UpdateWorkflowVersionDescriptionByWanwu(ctx context.Context, req *UpdateWorkflowVersionDescriptionRequest) (_ *workflow.UpdateWorkflowMetaResponse, err error) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			err = safego.NewPanicErr(panicErr, debug.Stack())
		}
	}()
	err = GetWorkflowDomainSVC().UpdateWorkflowVersionDescriptionByWanwu(ctx, mustParseInt64(req.WorkflowID), req.VersionDescription)
	if err != nil {
		return nil, err
	}
	return &workflow.UpdateWorkflowMetaResponse{}, nil
}

func (w *ApplicationService) RollbackWorkflowVersionByWanwu(ctx context.Context, req *RollbackWorkflowVersionRequest) (
	_ *workflow.SaveWorkflowResponse, err error,
) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			err = safego.NewPanicErr(panicErr, debug.Stack())
		}

		if err != nil {
			err = vo.WrapIfNeeded(errno.ErrWorkflowOperationFail, err, errorx.KV("cause", vo.UnwrapRootErr(err).Error()))
		}
	}()

	var version string
	if req.CommitID != "" {
		if idx := strings.LastIndex(req.CommitID, "_"); idx != -1 && idx < len(req.CommitID)-1 {
			version = req.CommitID[idx+1:]
		} else {
			return nil, fmt.Errorf("invalid commit_id format: %s", req.CommitID)
		}
	}
	policy := &vo.GetPolicy{
		ID:      mustParseInt64(req.WorkflowID),
		QType:   ternary.IFElse(len(version) > 0, workflowModel.FromSpecificVersion, workflowModel.FromDraft),
		Version: version,
	}

	wfEntity, err := GetWorkflowDomainSVC().Get(ctx, policy)
	if err != nil {
		return nil, err
	}
	if err := GetWorkflowDomainSVC().Save(ctx, mustParseInt64(req.WorkflowID), wfEntity.Canvas); err != nil {
		return nil, err
	}

	return &workflow.SaveWorkflowResponse{
		Data: &workflow.SaveWorkflowData{},
	}, nil
}

func (w *ApplicationService) GetWorkflowVersionSchemaByWanwu(ctx context.Context, req *workflow.GetHistorySchemaRequest) (
	resp *workflow.GetHistorySchemaResponse, err error) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			err = safego.NewPanicErr(panicErr, debug.Stack())
		}

		if err != nil {
			err = vo.WrapIfNeeded(errno.ErrWorkflowOperationFail, err, errorx.KV("cause", vo.UnwrapRootErr(err).Error()))
		}
	}()
	if req.GetSpaceID() == "" {
		return nil, vo.WrapError(errno.ErrInvalidParameter, fmt.Errorf("space_id is required"))
	}
	if req.GetWorkflowID() == "" {
		return nil, vo.WrapError(errno.ErrInvalidParameter, fmt.Errorf("workflow_id is required"))
	}

	var version string
	var policy *vo.GetPolicy
	policy = &vo.GetPolicy{
		ID:    mustParseInt64(req.GetWorkflowID()),
		QType: workflowModel.FromLatestVersion,
	}
	if req.CommitID != nil {
		commitID := *req.CommitID
		if idx := strings.LastIndex(commitID, "_"); idx != -1 && idx < len(commitID)-1 {
			version = commitID[idx+1:]
		} else {
			return nil, fmt.Errorf("invalid commit_id format: %s", commitID)
		}
		policy = &vo.GetPolicy{
			ID:      mustParseInt64(req.GetWorkflowID()),
			QType:   workflowModel.FromSpecificVersion,
			Version: version,
		}
	}
	wfEntity, err := GetWorkflowDomainSVC().Get(ctx, policy)
	return &workflow.GetHistorySchemaResponse{
		Data: &workflow.GetHistorySchemaData{
			Name:       wfEntity.Name,
			Describe:   wfEntity.Desc,
			URL:        wfEntity.IconURL,
			Schema:     wfEntity.Canvas,
			FlowMode:   wfEntity.Mode,
			WorkflowID: req.GetWorkflowID(),
			CommitID:   req.GetWorkflowID() + "_" + version,
		},
	}, nil
}

func (w *ApplicationService) MGetWorkflowLatestVersionByWanwu(ctx context.Context, req *MGetWorkflowLatestVersionRequest) (_ *MGetWorkflowLatestVersionResponse, err error) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			err = safego.NewPanicErr(panicErr, debug.Stack())
		}
	}()

	if len(req.WorkflowIDs) == 0 {
		return &MGetWorkflowLatestVersionResponse{
			Data: []*WorkflowVersionInfo{},
			Code: 0,
			Msg:  "success",
		}, nil
	}

	workflowIDs := make([]int64, 0, len(req.WorkflowIDs))
	for _, id := range req.WorkflowIDs {
		workflowIDs = append(workflowIDs, mustParseInt64(id))
	}

	versionMap, err := GetWorkflowDomainSVC().MGetWorkflowLatestVersionByWanwu(ctx, workflowIDs)
	if err != nil {
		return nil, err
	}

	response := &MGetWorkflowLatestVersionResponse{
		Data: make([]*WorkflowVersionInfo, 0, len(versionMap)),
		Code: 0,
		Msg:  "success",
	}

	for workflowID, v := range versionMap {
		wfID := strconv.FormatInt(workflowID, 10)
		if v != nil {
			response.Data = append(response.Data, &WorkflowVersionInfo{
				WorkflowID:         wfID,
				Version:            v.Version,
				VersionDescription: v.VersionDescription,
				CreatedAt:          v.VersionCreatedAt.UnixMilli(),
				CommitID:           wfID + "_" + v.Version,
				Type:               workflow.OperateType_PublishOperate,
			})
		}
	}

	return response, nil
}

// --- internal ---

type GetWorkflowVersionListResponse struct {
	Data     *WorkflowVersionListData `thrift:"data,1,required" form:"data,required" json:"data,required" query:"data,required"`
	Code     int64                    `thrift:"code,253,required" form:"code,required" json:"code,required" query:"code,required"`
	Msg      string                   `thrift:"msg,254,required" form:"msg,required" json:"msg,required" query:"msg,required"`
	BaseResp *base.BaseResp           `thrift:"BaseResp,255,required" form:"BaseResp,required" json:"BaseResp,required" query:"BaseResp,required"`
}

type WorkflowVersionListData struct {
	WorkflowID  string             `thrift:"workflow_id,1" form:"workflow_id" json:"workflow_id" query:"workflow_id"`
	VersionList []*WorkflowVersion `thrift:"version_list,2" form:"version_list" json:"version_list" query:"version_list"`
	Total       int32              `thrift:"total,3" form:"total" json:"total" query:"total"`
}

type WorkflowVersion struct {
	Version            string               `thrift:"version,1" form:"version" json:"version" query:"version"`
	VersionDescription string               `thrift:"version_description,2" form:"version_description" json:"version_description" query:"version_description"`
	CreatedAt          int64                `thrift:"created_at,3" form:"created_at" json:"created_at" query:"created_at"`
	CommitID           string               `thrift:"commit_id,3,optional" form:"commit_id" json:"commit_id,omitempty" query:"commit_id"`
	Type               workflow.OperateType `thrift:"type,4,required" form:"type,required" json:"type,required" query:"type,required"`
}

type UpdateWorkflowVersionDescriptionRequest struct {
	WorkflowID         string `thrift:"workflow_id,1,required" form:"workflow_id,required" json:"workflow_id,required" query:"workflow_id,required"`
	VersionDescription string `thrift:"version_description,8,optional" form:"version_description" json:"version_description,omitempty" query:"version_description"`
}

type RollbackWorkflowVersionRequest struct {
	WorkflowID string  `thrift:"workflow_id,1,required" form:"workflow_id,required" json:"workflow_id,required" query:"workflow_id,required"`
	Version    *string `thrift:"version,6,optional" form:"version" json:"version,omitempty" query:"version"`
	CommitID   string  `thrift:"commit_id,3,optional" form:"commit_id" json:"commit_id,omitempty" query:"commit_id"`
}

type MGetWorkflowLatestVersionRequest struct {
	WorkflowIDs []string `thrift:"workflow_ids,1" form:"workflow_ids" json:"workflow_ids" query:"workflow_ids"`
}

type MGetWorkflowLatestVersionResponse struct {
	Data []*WorkflowVersionInfo `thrift:"data,1,required" form:"data,required" json:"data,required" query:"data,required"`
	Code int64                  `thrift:"code,253,required" form:"code,required" json:"code,required" query:"code,required"`
	Msg  string                 `thrift:"msg,254,required" form:"msg,required" json:"msg,required" query:"msg,required"`
}

type WorkflowVersionInfo struct {
	WorkflowID         string               `thrift:"workflow_id,1" form:"workflow_id" json:"workflow_id" query:"workflow_id"`
	Version            string               `thrift:"version,2" form:"version" json:"version" query:"version"`
	VersionDescription string               `thrift:"version_description,3" form:"version_description" json:"version_description" query:"version_description"`
	CreatedAt          int64                `thrift:"created_at,4" form:"created_at" json:"created_at" query:"created_at"`
	CommitID           string               `thrift:"commit_id,5" form:"commit_id" json:"commit_id" query:"commit_id"`
	Type               workflow.OperateType `thrift:"type,6" form:"type" json:"type" query:"type"`
}
