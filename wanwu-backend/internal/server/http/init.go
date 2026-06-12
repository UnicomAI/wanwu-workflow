package http

import (
	"context"

	hertz_server "github.com/cloudwego/hertz/pkg/app/server"
	hertz_config "github.com/cloudwego/hertz/pkg/common/config"
	coze_middleware "github.com/coze-dev/coze-studio/backend/api/middleware"
	trace_util "github.com/coze-dev/coze-studio/backend/pkg/trace-util"
	hertz_cors "github.com/hertz-contrib/cors"
	hertztracing "github.com/hertz-contrib/obs-opentelemetry/tracing"

	"github.com/UnicomAI/wanwu-workflow/wanwu-backend/config"
	"github.com/UnicomAI/wanwu-workflow/wanwu-backend/internal/server/http/middleware"
	"github.com/UnicomAI/wanwu-workflow/wanwu-backend/internal/server/http/router"
)

func Init() {
	tracer, tracerCfg := hertztracing.NewServerTracer()
	opts := []hertz_config.Option{
		hertz_server.WithHostPorts(config.Cfg().Server.HttpEndpoint),
		hertz_server.WithMaxRequestBodySize(config.Cfg().Server.MaxReqBodySize),
		tracer,
	}

	s := hertz_server.Default(opts...)

	// Register trace shutdown hook (Hertz calls this on graceful shutdown)
	s.OnShutdown = append(s.OnShutdown, func(ctx context.Context) {
		trace_util.ShutdownTracer(ctx)
	})

	// cors option
	corsCfg := hertz_cors.DefaultConfig()
	corsCfg.AllowAllOrigins = true
	corsCfg.AllowHeaders = []string{"*"}
	corsHandler := hertz_cors.New(corsCfg)

	// Middleware order matters
	s.Use(coze_middleware.ContextCacheMW())     // must be first
	s.Use(coze_middleware.RequestInspectorMW()) // must be second
	s.Use(middleware.SetHost)
	s.Use(coze_middleware.SetLogIDMW())
	s.Use(corsHandler)
	s.Use(coze_middleware.AccessLogMW())
	s.Use(coze_middleware.OpenapiAuthMW())
	// s.Use(coze_middleware.SessionAuthMW())
	s.Use(middleware.I18n)
	s.Use(middleware.JwtUser)                       // must after I18n
	s.Use(middleware.SetUserID)                     // set userID
	s.Use(middleware.SetOrgID)                      // set orgID
	s.Use(hertztracing.ServerMiddleware(tracerCfg)) //trace

	router.Register(s)
	s.Spin()
}
