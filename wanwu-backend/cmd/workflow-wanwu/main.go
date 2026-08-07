package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"runtime"

	"github.com/UnicomAI/wanwu/pkg/db"
	jwt_util "github.com/UnicomAI/wanwu/pkg/jwt-util"
	"github.com/UnicomAI/wanwu/pkg/log"
	coze_cache_impl "github.com/coze-dev/coze-studio/backend/infra/cache/impl/redis"
	coze_minio "github.com/coze-dev/coze-studio/backend/infra/storage/impl/minio"
	"github.com/coze-dev/coze-studio/backend/pkg/logs"
	"github.com/coze-dev/coze-studio/backend/pkg/safego"
	trace_util "github.com/coze-dev/coze-studio/backend/pkg/trace-util"
	"github.com/coze-dev/coze-studio/backend/types/consts"

	"github.com/UnicomAI/wanwu-workflow/wanwu-backend/config"
	wanwu_http "github.com/UnicomAI/wanwu-workflow/wanwu-backend/internal/server/http"
	"github.com/UnicomAI/wanwu-workflow/wanwu-backend/internal/service/workflow"
	"github.com/UnicomAI/wanwu-workflow/wanwu-backend/pkg/redis"
	"github.com/UnicomAI/wanwu-workflow/wanwu-backend/pkg/statistictrace"
)

var (
	configFile string
	isVersion  bool

	buildTime    string //编译时间
	buildVersion string //编译版本
	gitCommitID  string //git的commit id
	gitBranch    string //git branch
	builder      string //构建者
)

func main() {
	flag.StringVar(&configFile, "config", "configs/config.yaml", "conf yaml file")
	flag.BoolVar(&isVersion, "v", false, "build message")
	flag.Parse()

	if isVersion {
		versionPrint()
		return
	}

	ctx := context.Background()

	flag.Parse()
	if err := config.LoadConfig(configFile); err != nil {
		log.Fatalf("init cfg err: %v", err)
	}
	logs.SetLevel(logs.LevelTrace)

	if err := log.InitLog(config.Cfg().Log.Std, config.Cfg().Log.Level, config.Cfg().Log.Logs...); err != nil {
		log.Fatalf("init log err: %v", err)
	}

	if err := jwt_util.InitUserJWT(config.Cfg().JWT.SigningKey); err != nil {
		log.Fatalf("init jwt err: %v", err)
	}

	// init tracer
	if err := trace_util.InitTracer("workflow-service"); err != nil {
		log.Fatalf("init tracer err: %v", err)
	}

	// db
	dbCli, err := db.New(config.Cfg().DB)
	if err != nil {
		log.Fatalf("init db err: %v", err)
	}

	// redis (workflow 业务缓存，db=1)
	if err := redis.InitWorkflow(ctx, config.Cfg().Redis); err != nil {
		log.Fatalf("init redis err: %v", err)
	}
	redisCli := coze_cache_impl.NewWithRedisCli(redis.Workflow())

	// redis OP（与 BFF trace_user 共用 db=7，供模型统计 TraceInfo 写入）
	if err := statistictrace.InitOP(ctx, toStatisticRedisConfig(config.Cfg().Redis)); err != nil {
		log.Fatalf("init statistic trace redis OP err: %v", err)
	}

	// minio
	minioCli, err := coze_minio.New(ctx, config.Cfg().Minio.Endpoint, config.Cfg().Minio.User, config.Cfg().Minio.Password, config.WorkflowBucket, false)
	if err != nil {
		log.Fatalf("init minio err: %v", err)
	}

	// imagex
	imageXCli, err := coze_minio.NewStorageImagex(ctx, config.Cfg().Minio.Endpoint, config.Cfg().Minio.User, config.Cfg().Minio.Password, config.WorkflowBucket, false)
	if err != nil {
		log.Fatalf("init imagex err: %v", err)
	}

	// workflow
	if err := workflow.Init(ctx, workflow.Infra{DB: dbCli, Cache: redisCli, Storage: minioCli, ImageX: imageXCli}); err != nil {
		log.Fatalf("init workflow service err: %v", err)
	}

	// http
	asyncStartMinioProxyServer(ctx)
	wanwu_http.Init()
}

// ！！！同步于：backend/main.go
// ！！！同步于：backend/main.go
// ！！！同步于：backend/main.go

// asyncStartMinioProxyServer
// 1. workflow-wanwu生成的minio presigned url例如：http://minio-wanwu:9000/abc/def?ghi=jkl
// 2. 需要nginx:8081代理，因此返回给前端的url例如：http://<external_ip>:8081/workflow/minio/presign/abc/def?ghi=jkl
// 3. nginx:8081不能直接代理minio-wanwu:9000，因为前端展示给用户的url增加了 x-wf-file_name 的query，例如：http://<external_ip>:8081/workflow/minio/presign/abc/def?ghi=jkl&x-wf-file_name=example.png
// 4. 因此需要nginx:8081代理workflow-wanwu:8998，再由workflow-wanwu:8998将去掉 x-wf-file_name 的url代理至minio-wanwu:9000
func asyncStartMinioProxyServer(ctx context.Context) {
	storageType := getEnv(consts.StorageType, "minio")
	proxyURL := getEnv(consts.MinIOAPIHost, "http://localhost:9000")

	// if storageType == "tos" {
	// 	proxyURL = getEnv(consts.TOSBucketEndpoint, "https://opencoze.tos-cn-beijing.volces.com")
	// }

	if storageType == "s3" {
		proxyURL = getEnv(consts.S3BucketEndpoint, "")
	}

	minioProxyEndpoint := getEnv(consts.MinIOProxyEndpoint, "")
	if len(minioProxyEndpoint) == 0 {
		return
	}

	safego.Go(ctx, func() {
		target, err := url.Parse(proxyURL)
		if err != nil {
			log.Fatalf(err.Error())
		}

		proxy := httputil.NewSingleHostReverseProxy(target)
		originDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			q := req.URL.Query()
			q.Del("x-wf-file_name")
			req.URL.RawQuery = q.Encode()

			originDirector(req)
			req.Host = req.URL.Host
		}
		useSSL := getEnv(consts.UseSSL, "0")
		if useSSL == "1" {
			logs.Infof("Minio proxy server is listening on %s with SSL", minioProxyEndpoint)
			err := http.ListenAndServeTLS(minioProxyEndpoint,
				getEnv(consts.SSLCertFile, ""),
				getEnv(consts.SSLKeyFile, ""), proxy)
			if err != nil {
				log.Fatalf(err.Error())
			}
		} else {
			logs.Infof("Minio proxy server is listening on %s", minioProxyEndpoint)
			err := http.ListenAndServe(minioProxyEndpoint, proxy)
			if err != nil {
				log.Fatalf(err.Error())
			}
		}
	})
}

func getEnv(key string, defaultValue string) string {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue
	}
	return v
}

func toStatisticRedisConfig(cfg redis.Config) statistictrace.RedisConfig {
	return statistictrace.RedisConfig{
		Host:       cfg.Host,
		Port:       cfg.Port,
		Username:   cfg.Username,
		Password:   cfg.Password,
		Standalone: cfg.Standalone,
		MasterName: cfg.MasterName,
	}
}

func versionPrint() {
	fmt.Printf("build_time: %s\n", buildTime)
	fmt.Printf("build_version: %s\n", buildVersion)
	fmt.Printf("git_commit_id: %s\n", gitCommitID)
	fmt.Printf("git branch: %s\n", gitBranch)
	fmt.Printf("runtime version: %s\n", runtime.Version())
	fmt.Printf("builder: %s\n", builder)
}
