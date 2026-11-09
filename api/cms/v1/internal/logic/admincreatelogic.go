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

type AdminCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminCreateLogic {
	return &AdminCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminCreateLogic) AdminCreate(req *types.CreateAdminReq) (resp *types.CommonResp, err error) {
	// 检查用户是否存在
	exists, err := l.svcCtx.EntClient.AdminUser.
		Query().
		Where(adminuser.Username(req.Username)).
		Exist(l.ctx)

	if err != nil {
		logx.Errorf("查询管理员用户失败: %v", err)
		return nil, common.NewBizError(common.ErrInternalServer)
	}

	if exists {
		return nil, common.NewBizError(common.ErrAdminUserAlreadyExists)
	}

	// 检查邮箱是否存在
	if req.Email != "" {
		emailExists, err := l.svcCtx.EntClient.AdminUser.
			Query().
			Where(adminuser.EmailEQ(req.Email)).
			Exist(l.ctx)
		if err != nil {
			logx.Errorf("查询管理员邮箱失败: %v", err)
			return nil, common.NewBizError(common.ErrInternalServer)
		}
		if emailExists {
			return nil, common.NewBizError(common.ErrAdminEmailAlreadyExists)
		}
	}

	// 密码复杂度校验
	if ok, msg := utils.ValidatePasswordComplexity(req.Password); !ok {
		return nil, common.NewBizErrorWithMsg(common.ErrInvalidParams, msg)
	}

	// 密码加密
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		logx.Errorf("密码加密失败: %v", err)
		return nil, common.NewBizError(common.ErrAdminPasswordHashFail)
	}

	// 创建用户
	user, err := l.svcCtx.EntClient.AdminUser.
		Create().
		SetUsername(req.Username).
		SetPasswordHash(string(hashed)).
		SetNickname(req.Nickname).
		SetIsSuper(req.IsSuper).
		SetIsActive(req.IsActive).
		Save(l.ctx)

	if err != nil {
		logx.Errorf("创建管理员失败: %v", err)
		// 已被软删除的同名/同邮箱账号仍占用唯一索引，这里还原正确的业务语义
		if ent.IsConstraintError(err) {
			return nil, common.NewBizError(common.ErrAdminUserAlreadyExists)
		}
		return nil, common.NewBizError(common.ErrAdminCreateFailed)
	}

	return &types.CommonResp{ID: user.ID, Message: "创建成功"}, nil
}
