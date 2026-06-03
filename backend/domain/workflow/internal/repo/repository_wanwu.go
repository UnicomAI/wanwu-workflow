package repo

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	einoCompose "github.com/cloudwego/eino/compose"
	"github.com/coze-dev/coze-studio/backend/bizpkg/llm/modelbuilder"
	"github.com/coze-dev/coze-studio/backend/domain/workflow"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/entity/vo"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/internal/repo/dal/query"
	"github.com/coze-dev/coze-studio/backend/infra/cache"
	"github.com/coze-dev/coze-studio/backend/infra/idgen"
	"github.com/coze-dev/coze-studio/backend/infra/storage"
	"github.com/coze-dev/coze-studio/backend/pkg/errorx"
	"github.com/coze-dev/coze-studio/backend/types/errno"
	"gorm.io/gorm"
)

// NewRepositoryWanwu 参考NewRepository，去掉chatModel初始化Suggester
func NewRepositoryWanwu(idgen idgen.IDGenerator, db *gorm.DB, redis cache.Cmdable, tos storage.Storage,
	cpStore einoCompose.CheckPointStore, chatModel modelbuilder.BaseChatModel, workflowConfig workflow.WorkflowConfig) (workflow.Repository, error) {
	return &RepositoryImpl{
		IDGenerator:     idgen,
		query:           query.Use(db),
		redis:           redis,
		tos:             tos,
		CheckPointStore: cpStore,
		InterruptEventStore: &interruptEventStoreImpl{
			redis: redis,
		},
		CancelSignalStore: &cancelSignalStoreImpl{
			redis: redis,
		},
		ExecuteHistoryStore: &executeHistoryStoreImplByWanwu{
			executeHistoryStoreImpl: &executeHistoryStoreImpl{
				query: query.Use(db),
				redis: redis,
			},
		},

		builtinModel:   chatModel,
		Suggester:      nil,
		WorkflowConfig: workflowConfig,
	}, nil

}

func (r *RepositoryImpl) GetVersionListByWanwu(ctx context.Context, id int64) (_ []*vo.VersionInfo, err error) {
	defer func() {
		if err != nil {
			err = vo.WrapIfNeeded(errno.ErrDatabaseError, err)
		}
	}()

	wfVersions, err := r.query.WorkflowVersion.WithContext(ctx).
		Where(r.query.WorkflowVersion.WorkflowID.Eq(id)).
		Order(r.query.WorkflowVersion.CreatedAt.Desc()).
		Find()
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow version list for ID %d: %w", id, err)
	}

	if len(wfVersions) == 0 {
		return []*vo.VersionInfo{}, nil
	}
	versionInfos := make([]*vo.VersionInfo, 0, len(wfVersions))
	for _, wfVersion := range wfVersions {
		versionInfo := &vo.VersionInfo{
			VersionMeta: &vo.VersionMeta{
				Version:            wfVersion.Version,
				VersionDescription: wfVersion.VersionDescription,
				VersionCreatedAt:   time.UnixMilli(wfVersion.CreatedAt),
			},
			CommitID: wfVersion.CommitID,
		}
		versionInfos = append(versionInfos, versionInfo)
	}

	return versionInfos, nil
}

func (r *RepositoryImpl) UpdateWorkflowVersionDescriptionByWanwu(ctx context.Context, id int64, versionDescription string) (err error) {
	defer func() {
		if err != nil {
			err = vo.WrapIfNeeded(errno.ErrDatabaseError, err)
		}
	}()

	meta, err := r.query.WorkflowMeta.WithContext(ctx).
    Select(r.query.WorkflowMeta.LatestVersion).
    Where(r.query.WorkflowMeta.ID.Eq(id)).
    First()

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return vo.WrapError(errno.ErrWorkflowNotFound,
				fmt.Errorf("workflow meta not found for ID %d", id),
				errorx.KV("workflowID", strconv.FormatInt(id, 10)))
		}
		return fmt.Errorf("failed to get latest_version from workflow_meta for ID %d: %w", id, err)
	}
	if meta.LatestVersion == "" {
		return vo.WrapError(errno.ErrWorkflowNotFound,
			fmt.Errorf("no latest version recorded for workflow ID %d", id),
			errorx.KV("workflowID", strconv.FormatInt(id, 10)))
	}
	_, err = r.query.WorkflowVersion.WithContext(ctx).
		Where(r.query.WorkflowVersion.WorkflowID.Eq(id)).
		Where(r.query.WorkflowVersion.Version.Eq(meta.LatestVersion)).
		Update(r.query.WorkflowVersion.VersionDescription, versionDescription)

	if err != nil {
		return fmt.Errorf("failed to update version description for workflow ID %d, version %s: %w",
			id, meta.LatestVersion, err)
	}
	return nil
}

func (r *RepositoryImpl) MGetWorkflowLatestVersionByWanwu(ctx context.Context, workflowIDs []int64) (_ map[int64]*vo.VersionInfo, err error) {
	defer func() {
		if err != nil {
			err = vo.WrapIfNeeded(errno.ErrDatabaseError, err)
		}
	}()

	if len(workflowIDs) == 0 {
		return make(map[int64]*vo.VersionInfo), nil
	}

	metas, err := r.query.WorkflowMeta.WithContext(ctx).
		Where(r.query.WorkflowMeta.ID.In(workflowIDs...)).
		Select(r.query.WorkflowMeta.ID, r.query.WorkflowMeta.LatestVersion).
		Find()
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow meta for IDs %v: %w", workflowIDs, err)
	}

	result := make(map[int64]*vo.VersionInfo)
	if len(metas) == 0 {
		return result, nil
	}

	latestVersions := make([]struct {
		WorkflowID int64
		Version    string
	}, 0, len(metas))
	for _, m := range metas {
		if m.LatestVersion != "" {
			latestVersions = append(latestVersions, struct {
				WorkflowID int64
				Version    string
			}{WorkflowID: m.ID, Version: m.LatestVersion})
		}
	}

	if len(latestVersions) == 0 {
		return result, nil
	}

	versionMap := make(map[int64]string)
	for _, lv := range latestVersions {
		versionMap[lv.WorkflowID] = lv.Version
	}

	wfVersions, err := r.query.WorkflowVersion.WithContext(ctx).
		Where(r.query.WorkflowVersion.WorkflowID.In(workflowIDs...)).
		Find()
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow versions for IDs %v: %w", workflowIDs, err)
	}

	for _, wfVersion := range wfVersions {
		if expectedVersion, ok := versionMap[wfVersion.WorkflowID]; ok && wfVersion.Version == expectedVersion {
			result[wfVersion.WorkflowID] = &vo.VersionInfo{
				VersionMeta: &vo.VersionMeta{
					Version:            wfVersion.Version,
					VersionDescription: wfVersion.VersionDescription,
					VersionCreatedAt:   time.UnixMilli(wfVersion.CreatedAt),
				},
				CommitID: wfVersion.CommitID,
			}
		}
	}

	return result, nil
}
