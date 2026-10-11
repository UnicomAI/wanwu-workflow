package middleware

import (
	"context"
	"os"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/UnicomAI/wanwu-workflow/wanwu-backend/config"
	svc_auth "github.com/UnicomAI/wanwu-workflow/wanwu-backend/pkg/svc-auth"
)

func InternalAuth(ctx context.Context, appCtx *app.RequestContext) {
	secret := os.Getenv("WANWU_INTERNAL_AUTH_SECRET")
	if secret == "" {
		appCtx.Next(ctx)
		return
	}
	userID := appCtx.Request.Header.Get(config.X_USER_ID)
	orgID := appCtx.Request.Header.Get(config.X_ORG_ID)
	ts := appCtx.Request.Header.Get(svc_auth.HeaderTS)
	sig := appCtx.Request.Header.Get(svc_auth.HeaderSign)
	if !svc_auth.Verify(secret, userID, orgID, ts, sig) {
		appCtx.Request.Header.Del(config.X_USER_ID)
		appCtx.Request.Header.Del(config.X_ORG_ID)
	}
	appCtx.Next(ctx)
}
