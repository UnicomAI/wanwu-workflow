package service

import (
	"context"

	einoCompose "github.com/cloudwego/eino/compose"
	"github.com/coze-dev/coze-studio/backend/bizpkg/llm/modelbuilder"
	"github.com/coze-dev/coze-studio/backend/domain/workflow"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/entity/vo"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/internal/canvas/adaptor"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/internal/repo"
	"github.com/coze-dev/coze-studio/backend/infra/cache"
	"github.com/coze-dev/coze-studio/backend/infra/idgen"
	"github.com/coze-dev/coze-studio/backend/infra/storage"
	"gorm.io/gorm"
)

// RegisterWanwuAllNodeAdaptors 参考RegisterAllNodeAdaptors
func RegisterWanwuAllNodeAdaptors() {
	adaptor.RegisterWanwuAllNodeAdaptors()
}

// NewWorkflowRepositoryWanwu 参考NewWorkflowRepository，去掉chatModel初始化Suggester
func NewWorkflowRepositoryWanwu(idgen idgen.IDGenerator, db *gorm.DB, redis cache.Cmdable, tos storage.Storage,
	cpStore einoCompose.CheckPointStore, chatModel modelbuilder.BaseChatModel, cfg workflow.WorkflowConfig) (workflow.Repository, error) {
	return repo.NewRepositoryWanwu(idgen, db, redis, tos, cpStore, chatModel, cfg)
}

func (i *impl) GetWorkflowVersionListByWanwu(ctx context.Context, workflowID int64) ([]*vo.VersionInfo, error) {
    return i.repo.GetVersionListByWanwu(ctx, workflowID)
}

func (i *impl) UpdateWorkflowVersionDescriptionByWanwu(ctx context.Context, workflowID int64, versionDescription string) error {
    return i.repo.UpdateWorkflowVersionDescriptionByWanwu(ctx, workflowID, versionDescription)
}

func (i *impl) MGetWorkflowLatestVersionByWanwu(ctx context.Context, workflowIDs []int64) (map[int64]*vo.VersionInfo, error) {
    return i.repo.MGetWorkflowLatestVersionByWanwu(ctx, workflowIDs)
}
