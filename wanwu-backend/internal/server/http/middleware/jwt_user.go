package middleware

import (
	"context"
	"fmt"
	"strings"
	"time"

	jwt_util "github.com/UnicomAI/wanwu/pkg/jwt-util"
	"github.com/UnicomAI/wanwu/pkg/util"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/coze-dev/coze-studio/backend/api/middleware"
	"github.com/coze-dev/coze-studio/backend/domain/user/entity"
	"github.com/coze-dev/coze-studio/backend/pkg/ctxcache"
	"github.com/coze-dev/coze-studio/backend/pkg/errorx"
	"github.com/coze-dev/coze-studio/backend/pkg/i18n"
	"github.com/coze-dev/coze-studio/backend/pkg/logs"
	"github.com/coze-dev/coze-studio/backend/types/consts"
	"github.com/coze-dev/coze-studio/backend/types/errno"

	"github.com/UnicomAI/wanwu-workflow/wanwu-backend/pkg/httputil"
)

func JwtUser(ctx context.Context, appCtx *app.RequestContext) {
	requestAuthType := appCtx.GetInt32(middleware.RequestAuthTypeStr)
	if requestAuthType != int32(middleware.RequestAuthTypeWebAPI) {
		appCtx.Next(ctx)
		return
	}

	token, err := getJWTToken(appCtx)
	if err != nil {
		// httputil.Unauthorized(ctx, appCtx, errorx.New(errno.ErrUserAuthenticationFailed, errorx.KV("reason", err.Error())))
		// 可能是内部调用，没有jwt token；或者是前缀为 pat_ 开头的coze openapi token；或者是前缀为 ww- 开头的wanwu openapi token
		logs.CtxWarnf(ctx, "request (%v) check jwt token err: %v", string(appCtx.Request.Path()), err)
		appCtx.Next(ctx)
		return
	}

	claims, err := jwt_util.ParseToken(token)
	if err != nil {
		httputil.Unauthorized(ctx, appCtx, errorx.New(errno.ErrUserAuthenticationFailed, errorx.KV("reason", err.Error())))
		return
	}
	if claims.Subject != jwt_util.SUBJECT_USER {
		httputil.Unauthorized(ctx, appCtx, errorx.New(errno.ErrUserAuthenticationFailed, errorx.KV("reason", "invalid token subject")))
		return
	}

	ctxcache.Store(ctx, consts.SessionDataKeyInCtx, &entity.Session{
		UserID:    util.MustI64(claims.UserID),
		Locale:    string(i18n.GetLocale(ctx)),
		CreatedAt: time.Unix(claims.NotBefore, 0),
		ExpiresAt: time.Unix(claims.ExpiresAt, 0),
	})
	appCtx.Next(ctx)
}

// 从Header Authorization中获取Token
func getJWTToken(ctx *app.RequestContext) (token string, err error) {
	authorization := ctx.Request.Header.Get("Authorization")
	if authorization != "" {
		tks := strings.Split(authorization, " ")
		if len(tks) > 1 && tks[0] == "Bearer" {
			if strings.HasPrefix(tks[1], "pat_") || strings.HasPrefix(tks[1], "ww-") {
				return "", fmt.Errorf("coze openapi token (pat_)")
			}
			return tks[1], err
		} else {
			return "", fmt.Errorf("not Bearer token format")
		}
	} else {
		return "", fmt.Errorf("token is nil")
	}
}
