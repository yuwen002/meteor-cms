// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/adminuser"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/yuwen002/meteor-cms/internal/utils"
	"golang.org/x/crypto/bcrypt"

	"github.com/zeromicro/go-zero/core/logx"
)

type RegisterAdminLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRegisterAdminLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterAdminLogic {
	return &RegisterAdminLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RegisterAdminLogic) RegisterAdmin(req *types.RegisterReq) (resp *types.RegisterResp, err error) {
	// 1. 检查用户名是否存在
	exists, err := l.svcCtx.EntClient.AdminUser.Query().
		Where(adminuser.UsernameEQ(req.Username)).
		Exist(l.ctx)

	if err != nil {
		l.Logger.Errorf("检查用户失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	if exists {
		return nil, common.NewBizError(common.ErrAdminUserAlreadyExists)
	}

	// 2. 检查邮箱是否存在
	emailExists, err := l.svcCtx.EntClient.AdminUser.Query().
		Where(adminuser.EmailEQ(req.Email)).
		Exist(l.ctx)

	if err != nil {
		l.Logger.Errorf("检查邮箱失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}
	if emailExists {
		return nil, common.NewBizError(common.ErrAdminEmailAlreadyExists)
	}

	// 密码复杂度校验
	if ok, msg := utils.ValidatePasswordComplexity(req.Password); !ok {
		return nil, common.NewBizErrorWithMsg(common.ErrInvalidParams, msg)
	}

	// 3. 加密密码
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		l.Logger.Errorf("密码加密失败: %v", err)
		return nil, common.NewBizError(common.ErrAdminPasswordHashFail)
	}

	// 4. 创建用户
	_, err = l.svcCtx.EntClient.AdminUser.
		Create().
		SetUsername(req.Username).
		SetPasswordHash(string(hashed)).
		SetNickname(req.Nickname).
		SetEmail(req.Email).
		SetIsSuper(false).
		SetIsActive(true).
		Save(l.ctx)

	if err != nil {
		l.Logger.Errorf("创建管理员失败: %v", err)
		// 已被软删除的账号仍占用 username/email 唯一索引
		if ent.IsConstraintError(err) {
			return nil, common.NewBizError(common.ErrAdminUserAlreadyExists)
		}
		return nil, common.NewBizError(common.ErrAdminCreateFailed)
	}

	return &types.RegisterResp{
		Message: "管理员注册成功",
	}, nil
}
