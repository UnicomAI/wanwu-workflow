package workflow

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/UnicomAI/wanwu/pkg/log"
	"github.com/cloudwego/eino/callbacks"
	coze_app_conversation "github.com/coze-dev/coze-studio/backend/application/conversation"
	coze_app_memory "github.com/coze-dev/coze-studio/backend/application/memory"
	coze_app_openauth "github.com/coze-dev/coze-studio/backend/application/openauth"
	coze_app_upload "github.com/coze-dev/coze-studio/backend/application/upload"
	coze_app_user "github.com/coze-dev/coze-studio/backend/application/user"
	coze_app_workflow "github.com/coze-dev/coze-studio/backend/application/workflow"
	coze_cross_agentrun "github.com/coze-dev/coze-studio/backend/crossdomain/agentrun"
	coze_cross_agentrun_impl "github.com/coze-dev/coze-studio/backend/crossdomain/agentrun/impl"
	coze_cross_conversation "github.com/coze-dev/coze-studio/backend/crossdomain/conversation"
	coze_cross_conversation_impl "github.com/coze-dev/coze-studio/backend/crossdomain/conversation/impl"
	coze_cross_database "github.com/coze-dev/coze-studio/backend/crossdomain/database"
	coze_cross_database_impl "github.com/coze-dev/coze-studio/backend/crossdomain/database/impl"
	coze_cross_message "github.com/coze-dev/coze-studio/backend/crossdomain/message"
	coze_cross_message_impl "github.com/coze-dev/coze-studio/backend/crossdomain/message/impl"
	coze_cross_upload "github.com/coze-dev/coze-studio/backend/crossdomain/upload"
	coze_cross_upload_impl "github.com/coze-dev/coze-studio/backend/crossdomain/upload/impl"
	coze_cross_user "github.com/coze-dev/coze-studio/backend/crossdomain/user"
	coze_cross_workflow "github.com/coze-dev/coze-studio/backend/crossdomain/workflow"
	coze_cross_workflow_impl "github.com/coze-dev/coze-studio/backend/crossdomain/workflow/impl"
	coze_conversation_agentrun_repo "github.com/coze-dev/coze-studio/backend/domain/conversation/agentrun/repository"
	coze_conversation_agentrun "github.com/coze-dev/coze-studio/backend/domain/conversation/agentrun/service"
	coze_conversation_conversation_repo "github.com/coze-dev/coze-studio/backend/domain/conversation/conversation/repository"
	coze_conversation_conversation "github.com/coze-dev/coze-studio/backend/domain/conversation/conversation/service"
	coze_conversation_message_repo "github.com/coze-dev/coze-studio/backend/domain/conversation/message/repository"
	coze_conversation_message "github.com/coze-dev/coze-studio/backend/domain/conversation/message/service"
	coze_memory_database_service "github.com/coze-dev/coze-studio/backend/domain/memory/database/service"
	coze_upload_service "github.com/coze-dev/coze-studio/backend/domain/upload/service"
	coze_workflow "github.com/coze-dev/coze-studio/backend/domain/workflow"
	coze_workflow_service "github.com/coze-dev/coze-studio/backend/domain/workflow/service"
	coze_infra_cache "github.com/coze-dev/coze-studio/backend/infra/cache"
	coze_infra_checkpoint "github.com/coze-dev/coze-studio/backend/infra/checkpoint"
	coze_infra_code "github.com/coze-dev/coze-studio/backend/infra/coderunner"
	coze_infra_code_impl "github.com/coze-dev/coze-studio/backend/infra/coderunner/impl"
	coze_infra_idgen "github.com/coze-dev/coze-studio/backend/infra/idgen/impl/idgen"
	coze_infra_imagex "github.com/coze-dev/coze-studio/backend/infra/imagex"
	coze_infra_rdb_impl "github.com/coze-dev/coze-studio/backend/infra/rdb/impl/rdb"
	coze_infra_sqlparser "github.com/coze-dev/coze-studio/backend/infra/sqlparser"
	coze_infra_sqlparser_impl "github.com/coze-dev/coze-studio/backend/infra/sqlparser/impl/sqlparser"
	coze_infra_storage "github.com/coze-dev/coze-studio/backend/infra/storage"
	"gorm.io/gorm"

	"github.com/UnicomAI/wanwu-workflow/wanwu-backend/config"
	crosssearchImpl "github.com/UnicomAI/wanwu-workflow/wanwu-backend/internal/service/crossdomain/impl/search"
	crossuserImpl "github.com/UnicomAI/wanwu-workflow/wanwu-backend/internal/service/crossdomain/impl/user"
)

const (
	workflowRepoSqlFile         = "configs/schema.sql"
	workflowRepoSqlMigrationDir = "configs/migrations"
)

var _workflowService coze_workflow.Service

type Infra struct {
	DB      *gorm.DB
	Cache   coze_infra_cache.Cmdable
	Storage coze_infra_storage.Storage
	ImageX  coze_infra_imagex.ImageX
}

func Init(ctx context.Context, infra Infra) error {
	if _workflowService != nil {
		return errors.New("already init")
	}

	// init repo data
	if err := initRepo(infra.DB); err != nil {
		return fmt.Errorf("init repo err: %v", err)
	}
	// migrate repo
	if err := migrateRepo(infra.DB); err != nil {
		return fmt.Errorf("migrate repo err: %v", err)
	}

	// register all node adaptors
	coze_workflow_service.RegisterAllNodeAdaptors()
	coze_workflow_service.RegisterWanwuAllNodeAdaptors()

	// id generator
	idGen, _ := coze_infra_idgen.New(infra.Cache)
	// check point store
	cps := coze_infra_checkpoint.NewRedisStore(infra.Cache)
	// code runner
	coze_infra_code.SetCodeRunner(coze_infra_code_impl.NewByWanwu())
	// workflow repo
	workflowRepo, _ := coze_workflow_service.NewWorkflowRepositoryWanwu(idGen, infra.DB, infra.Cache, infra.Storage, cps, nil, config.Cfg().Workflow)
	coze_workflow.SetRepository(workflowRepo)
	coze_infra_sqlparser.New = coze_infra_sqlparser_impl.NewSQLParser
	// domain workflow service
	_workflowService = coze_workflow_service.NewWorkflowService(workflowRepo)
	// domain database service
	rdbSVC := coze_infra_rdb_impl.NewService(infra.DB, idGen)
	databaseDomainSVC := coze_memory_database_service.NewService(rdbSVC, infra.DB, idGen, infra.Storage, infra.Cache)
	// init application upload
	coze_app_upload.InitService(&coze_app_upload.UploadComponents{Cache: infra.Cache, Oss: infra.Storage, DB: infra.DB, Idgen: idGen})
	// init application user
	_ = coze_app_user.InitService(ctx, infra.DB, infra.Storage, idGen)
	// init application auth
	_ = coze_app_openauth.InitService(infra.DB, idGen)
	// init application workflow
	coze_app_workflow.SVC.DomainSVC = _workflowService
	coze_app_workflow.SVC.ImageX = infra.ImageX
	coze_app_workflow.SVC.TosClient = infra.Storage
	coze_app_workflow.SVC.IDGenerator = idGen
	coze_app_workflow.SetEventBus(crosssearchImpl.DefaultResourceEventBusMock())
	// init application memory
	coze_app_memory.DatabaseApplicationSVC.DomainSVC = databaseDomainSVC
	coze_app_memory.SetEventBus(crosssearchImpl.DefaultResourceEventBusMock())
	// init domain conversation conversation
	c := coze_conversation_conversation.NewService(&coze_conversation_conversation.Components{
		ConversationRepo: coze_conversation_conversation_repo.NewConversationRepo(infra.DB, idGen),
	})
	coze_cross_conversation.SetDefaultSVC(coze_cross_conversation_impl.InitDomainService(c))
	coze_app_conversation.ConversationSVC.ConversationDomainSVC = c
	// init domain conversation agentrun
	coze_cross_agentrun.SetDefaultSVC(coze_cross_agentrun_impl.InitDomainService(coze_conversation_agentrun.NewService(&coze_conversation_agentrun.Components{
		RunRecordRepo: coze_conversation_agentrun_repo.NewRunRecordRepo(infra.DB, idGen),
		ImagexSVC:     infra.ImageX,
	})))
	// init domain conversation message
	m := coze_conversation_message.NewService(&coze_conversation_message.Components{
		MessageRepo: coze_conversation_message_repo.NewMessageRepo(infra.DB, idGen),
	})
	coze_cross_message.SetDefaultSVC(coze_cross_message_impl.InitDomainService(m))
	coze_app_conversation.ConversationSVC.MessageDomainSVC = m
	cozeUploadSVC := coze_upload_service.NewUploadSVC(infra.DB, idGen, infra.Storage)
	coze_app_conversation.OpenapiMessageSVC.UploaodDomainSVC = cozeUploadSVC
	// init cross domain user
	coze_cross_user.SetDefaultSVC(crossuserImpl.DefaultMock())
	// init cross domain upload
	coze_cross_upload.SetDefaultWanwuSVC(coze_cross_upload_impl.NewWanwuUploader(infra.Storage))
	coze_cross_upload.SetDefaultSVC(coze_cross_upload_impl.InitDomainService(cozeUploadSVC))
	// init cross domain workflow
	coze_cross_workflow.SetDefaultSVC(coze_cross_workflow_impl.InitDomainService(_workflowService))
	// init cross domain database
	coze_cross_database.SetDefaultSVC(coze_cross_database_impl.InitDomainService(databaseDomainSVC))
	// init token callback handler
	callbacks.AppendGlobalHandlers(coze_workflow_service.GetTokenCallbackHandler())

	return nil
}

func initRepo(db *gorm.DB) error {
	return executeSqlFile(db, workflowRepoSqlFile, true)

}

func migrateRepo(db *gorm.DB) error {
	// 读取migrations中的sql文件
	if err := filepath.Walk(workflowRepoSqlMigrationDir, func(filePath string, fileInfo fs.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if fileInfo.IsDir() {
			return nil
		}
		return executeSqlFile(db, filePath, true)
	}); err != nil {
		return err
	}
	return nil
}

func executeSqlFile(db *gorm.DB, sqlFilePath string, skipExecErr bool) error {
	// 读取sql文件
	file, err := os.Open(sqlFilePath)
	if err != nil {
		return fmt.Errorf("open %v err: %v", sqlFilePath, err)
	}
	defer func() { _ = file.Close() }()
	// 过滤注释行
	var b []byte
	reader := bufio.NewReader(file)
	for {
		line, _, err := reader.ReadLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read %v err: %v", sqlFilePath, err)
		}

		lineStr := strings.TrimSpace(string(line))
		if strings.HasPrefix(lineStr, "-- ") {
			// 跳过注释行
			continue
		}
		b = append(b, line...)
		b = append(b, byte('\n'))
	}
	// 分隔sql，执行
	sqls := strings.Split(string(b), ";\n")
	for _, sql := range sqls {
		if sql = strings.TrimSpace(sql); sql == "" {
			continue
		}
		if err := db.Exec(sql).Error; err != nil {
			if !skipExecErr {
				return fmt.Errorf("execute [%v] err: %v", sql, err)
			}
			log.Warnf("execute [%v] err: %v", sql, err)
		}
	}
	return nil
}
