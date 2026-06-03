package wanwu_mock

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/coze-dev/coze-studio/backend/api/model/playground"

	crossuser "github.com/UnicomAI/wanwu-workflow/wanwu-backend/internal/service/crossdomain/impl/user"
	"github.com/UnicomAI/wanwu-workflow/wanwu-backend/pkg/httputil"
)

// GetSpaceListV2 .
// @router /api/playground_api/space/list [POST]
func GetSpaceListV2(ctx context.Context, c *app.RequestContext) {
	var err error
	var req playground.GetSpaceListV2Request
	err = c.BindAndValidate(&req)
	if err != nil {
		httputil.BadRequest(c, err.Error())
		return
	}

	resp, err := crossuser.DefaultMock().GetSpaceListV2(ctx, &req)
	if err != nil {
		httputil.InternalError(ctx, c, err)
		return
	}

	c.JSON(consts.StatusOK, resp)
}
