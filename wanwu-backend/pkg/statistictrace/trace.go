package statistictrace

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	traceUserPrefix = "trace_user:"
	traceUserTTL    = 30 * time.Minute

	TraceExtraSource             = "source"
	TraceExtraModule             = "module"
	TraceExtraModuleResourceID   = "moduleResourceId"
	TraceExtraModuleResourceType = "moduleResourceType"
	TraceExtraModuleCreatorUser  = "moduleCreatorUserId"
	TraceExtraModuleCreatorOrg   = "moduleCreatorOrgId"

	AppStatisticSourceWeb   = "web"
	StatisticModuleWorkflow = "workflow" // 对齐 wanwu BizModuleAppWorkflow
	AppTypeWorkflow         = "workflow"
	AppTypeChatflow         = "chatflow"
)

var _redisOP *redis.Client

// DraftRunTrace 草稿试运行写入 Redis 的统计字段。
type DraftRunTrace struct {
	APIPath       string
	WorkflowID    string
	AppType       string
	CallerUserID  string
	CallerOrgID   string
	CreatorUserID string
	CreatorOrgID  string
}

// InitOP 初始化与 BFF 模型统计共用的 Redis 客户端（db=7）。
func InitOP(ctx context.Context, cfg RedisConfig) error {
	if _redisOP != nil {
		return fmt.Errorf("statistic trace redis OP already init")
	}
	cli, err := newClient(ctx, cfg, 7)
	if err != nil {
		return err
	}
	_redisOP = cli
	return nil
}

func OP() *redis.Client {
	return _redisOP
}

func TraceUserKey(traceID string) string {
	return traceUserPrefix + traceID
}

// HasDraftRunTrace 是否已写入完整试运行/对话流统计信息。
func HasDraftRunTrace(ctx context.Context, traceID string) (bool, error) {
	rec, found, err := loadTraceRecord(ctx, traceID)
	if err != nil || !found {
		return false, err
	}
	return rec.Extra[TraceExtraSource] != "" &&
		rec.Extra[TraceExtraModule] != "" &&
		rec.Extra[TraceExtraModuleResourceID] != "", nil
}

// SaveDraftRunTrace 将试运行 app 统计字段写入 Redis trace_user:{traceId}。
// 内部用 protowire 编码，与 BFF proto.Unmarshal 读取格式兼容，不引入额外 .pb.go。
func SaveDraftRunTrace(ctx context.Context, traceID string, trace DraftRunTrace) error {
	rec, _, err := loadTraceRecord(ctx, traceID)
	if err != nil {
		return err
	}
	mergeTraceRecord(&rec, trace)
	rec.TraceID = traceID
	return _redisOP.Set(ctx, TraceUserKey(traceID), encodeTraceRecord(rec), traceUserTTL).Err()
}

// loadTraceRecord 读取并解析 Redis 已有的 trace 记录；key 不存在时 found=false。
func loadTraceRecord(ctx context.Context, traceID string) (rec traceRecord, found bool, err error) {
	if _redisOP == nil {
		return traceRecord{}, false, errors.New("statistic trace redis OP not initialized")
	}
	data, err := _redisOP.Get(ctx, TraceUserKey(traceID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return traceRecord{}, false, nil
	}
	if err != nil {
		return traceRecord{}, false, err
	}
	return decodeTraceRecord(data), true, nil
}
