package middleware

import (
	"context"

	"github.com/UnicomAI/wanwu/pkg/util"
	"github.com/cloudwego/hertz/pkg/app"
	openapi_entity "github.com/coze-dev/coze-studio/backend/domain/openauth/openapiauth/entity"
	user_entity "github.com/coze-dev/coze-studio/backend/domain/user/entity"
	"github.com/coze-dev/coze-studio/backend/pkg/ctxcache"
	"github.com/coze-dev/coze-studio/backend/pkg/i18n"
	"github.com/coze-dev/coze-studio/backend/types/consts"

	"github.com/UnicomAI/wanwu-workflow/wanwu-backend/config"
)

func SetUserID(ctx context.Context, appCtx *app.RequestContext) {
	// userID
	userID := appCtx.Request.Header.Get(config.X_USER_ID)
	if userID == "" {
		appCtx.Next(ctx)
		return
	}

	// session auth
	if _, ok := ctxcache.Get[*user_entity.Session](ctx, consts.SessionDataKeyInCtx); !ok {
		ctxcache.Store(ctx, consts.SessionDataKeyInCtx, &user_entity.Session{
			UserID: util.MustI64(userID),
			Locale: string(i18n.GetLocale(ctx)),
		})
	}

	// openapi auth
	if _, ok := ctxcache.Get[*openapi_entity.ApiKey](ctx, consts.OpenapiAuthKeyInCtx); !ok {
		ctxcache.Store(ctx, consts.OpenapiAuthKeyInCtx, &openapi_entity.ApiKey{
			UserID:      util.MustI64(userID),
			ConnectorID: consts.APIConnectorID,
		})
	}

	appCtx.Next(ctx)
}

func SetOrgID(ctx context.Context, appCtx *app.RequestContext) {
	// orgID
	orgID := appCtx.Request.Header.Get(config.X_ORG_ID)
	if orgID != "" {
		ctxcache.Store(ctx, config.X_ORG_ID, orgID)
	}
	appCtx.Next(ctx)
}
