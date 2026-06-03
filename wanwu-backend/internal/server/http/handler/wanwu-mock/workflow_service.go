package wanwu_mock

import (
	"context"
	"net/url"
	"os"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/coze-dev/coze-studio/backend/api/model/workflow"
	appworkflow "github.com/coze-dev/coze-studio/backend/application/workflow"

	"github.com/UnicomAI/wanwu-workflow/wanwu-backend/config"
	"github.com/UnicomAI/wanwu-workflow/wanwu-backend/pkg/httputil"
)

// NodeTemplateListByWanwu 参考NodeTemplateList
// @router /api/workflow_api/node_template_list [POST]
func NodeTemplateListByWanwu(ctx context.Context, c *app.RequestContext) {
	var err error
	var req workflow.NodeTemplateListRequest
	err = c.BindAndValidate(&req)
	if err != nil {
		httputil.BadRequest(c, err.Error())
		return
	}
	resp, err := appworkflow.SVC.GetNodeTemplateList(ctx, &req)
	if err != nil {
		httputil.BadRequest(c, err.Error())
		return
	}
	baseUrl := os.Getenv("WANWU_EXTERNAL_SCHEME") + "://" + os.Getenv("WANWU_EXTERNAL_ENDPOINT")
	for _, template := range resp.Data.TemplateList {
		for _, cfg := range config.Cfg().Icons {
			if template.ID == cfg.ID {
				url, _ := url.JoinPath(baseUrl, cfg.Url)
				template.IconURL = url
			}
		}
	}
	c.JSON(consts.StatusOK, resp)
}
