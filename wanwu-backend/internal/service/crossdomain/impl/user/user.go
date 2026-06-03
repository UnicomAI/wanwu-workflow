package user

import (
	"context"
	"fmt"

	"github.com/UnicomAI/wanwu/pkg/util"
	"github.com/coze-dev/coze-studio/backend/api/model/playground"
	"github.com/coze-dev/coze-studio/backend/application/base/ctxutil"
	"github.com/coze-dev/coze-studio/backend/domain/user/entity"
	"github.com/coze-dev/coze-studio/backend/pkg/ctxcache"
	"github.com/coze-dev/coze-studio/backend/pkg/lang/ptr"

	"github.com/UnicomAI/wanwu-workflow/wanwu-backend/config"
)

var defaultMock *mock = &mock{}

func DefaultMock() *mock {
	return defaultMock
}

type mock struct{}

func (u *mock) GetUserSpaceList(ctx context.Context, userID int64) (spaces []*entity.Space, err error) {
	orgID, ok := ctxcache.Get[string](ctx, config.X_ORG_ID)
	if !ok {
		return nil, nil
	}
	return []*entity.Space{
		{ID: util.MustI64(orgID)},
	}, nil
}

func (u *mock) GetSpaceListV2(ctx context.Context, req *playground.GetSpaceListV2Request) (
	resp *playground.GetSpaceListV2Response, err error,
) {
	userID := ctxutil.MustGetUIDFromCtx(ctx)
	spaces, _ := u.GetUserSpaceList(ctx, userID)
	var botSpaces []*playground.BotSpaceV2
	for _, space := range spaces {
		botSpaces = append(botSpaces, &playground.BotSpaceV2{
			ID:          space.ID,
			Name:        fmt.Sprintf("SPACE-%v", space.ID),
			Description: fmt.Sprintf("SPACE-%v-DESC", space.ID),
			SpaceType:   playground.SpaceType_Personal,
		})
	}
	return &playground.GetSpaceListV2Response{
		Data: &playground.SpaceInfo{
			BotSpaceList:          botSpaces,
			HasPersonalSpace:      true,
			TeamSpaceNum:          0,
			RecentlyUsedSpaceList: botSpaces,
			Total:                 ptr.Of(int32(len(botSpaces))),
			HasMore:               ptr.Of(false),
		},
	}, nil
}
