package logic

import (
	"context"

	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/adminuser"
	"github.com/yuwen002/meteor-cms/internal/utils"
	"github.com/zeromicro/go-zero/core/logx"
)

// currentUserIsSuper 从 DB 读取当前登录用户的 is_super，而非信任 token claim。
//
// 原因：token 中的 is_super 在管理员权限被调整、但旧 token 尚未过期期间仍携带旧值，
// 会导致「刚被取消超管的人仍可行使超管权限」或「刚被提权的人短期内不生效」的不一致。
// 取 DB 真值可保证实时一致（fail-closed：用户缺失 / 未找到视为非超管）。
// 调用方需确保请求已通过鉴权中间件（中间件已校验用户存在），故 NotFound 视为非超管。
func currentUserIsSuper(ctx context.Context, client *ent.Client) (bool, error) {
	userID, ok := utils.CurrentUserID(ctx)
	if !ok {
		return false, nil
	}

	user, err := client.AdminUser.
		Query().
		Where(adminuser.IDEQ(userID)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return false, nil
		}
		logx.Errorf("query current user is_super failed: user_id=%d err=%v", userID, err)
		return false, err
	}
	return user.IsSuper, nil
}
