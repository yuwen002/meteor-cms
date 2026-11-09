package seed

import (
	"context"
	"time"

	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/adminrole"
	"github.com/yuwen002/meteor-cms/ent/adminrolepermission"
	"github.com/yuwen002/meteor-cms/ent/adminuser"
	"github.com/yuwen002/meteor-cms/ent/adminuserrole"
	"github.com/yuwen002/meteor-cms/internal/utils"
)

// InitSeed 必须在单个事务里完成：中途失败时整体回滚，避免留下「有管理员没角色」
// 或「权限写了一半」这类半初始化状态。调用方负责提交或回滚。
func InitSeed(ctx context.Context, tx *ent.Tx) error {
	// 1️⃣ 初始化超级管理员用户
	user, err := initAdminUser(ctx, tx)
	if err != nil {
		return err
	}

	// 2️⃣ 初始化超级管理员角色
	role, err := initSuperAdminRole(ctx, tx)
	if err != nil {
		return err
	}

	// 3️⃣ 初始化默认权限
	perms, err := initDefaultPermissions(ctx, tx)
	if err != nil {
		return err
	}

	// 4️⃣ 内置系统配置与数据字典（幂等，不覆盖运营已改的值）
	if err := InitConfigs(ctx, tx); err != nil {
		return err
	}
	if err := InitDicts(ctx, tx); err != nil {
		return err
	}

	// 5️⃣ 用户角色关联（已存在则跳过，保证重复执行不报错）
	userRoleExists, err := tx.AdminUserRole.
		Query().
		Where(
			adminuserrole.UserID(user.ID),
			adminuserrole.RoleID(role.ID),
		).
		Exist(ctx)
	if err != nil {
		return err
	}
	if !userRoleExists {
		if _, err = tx.AdminUserRole.Create().
			SetUserID(user.ID).
			SetRoleID(role.ID).
			Save(ctx); err != nil {
			return err
		}
	}

	// 5️⃣ 角色权限关联（同上，幂等）
	for _, perm := range perms {
		if perm == nil {
			continue
		}

		rolePermExists, err := tx.AdminRolePermission.
			Query().
			Where(
				adminrolepermission.RoleID(role.ID),
				adminrolepermission.PermissionID(perm.ID),
			).
			Exist(ctx)
		if err != nil {
			return err
		}
		if rolePermExists {
			continue
		}

		if _, err = tx.AdminRolePermission.Create().
			SetRoleID(role.ID).
			SetPermissionID(perm.ID).
			Save(ctx); err != nil {
			return err
		}
	}

	return nil

}

// initAdminUser 初始化管理员用户
// 如果用户名 "admin" 已存在，则返回已存在的用户
// 如果不存在，则创建新的超级管理员用户，默认密码为 "123456"
// 返回创建或获取的管理员用户信息和可能的错误
func initAdminUser(ctx context.Context, tx *ent.Tx) (*ent.AdminUser, error) {
	exist, err := tx.AdminUser.Query().
		Where(adminuser.UsernameEQ("admin")).
		Exist(ctx)
	if err != nil {
		return nil, err
	}
	if exist {
		return tx.AdminUser.Query().Where(adminuser.UsernameEQ("admin")).Only(ctx)
	}
	password, err := utils.HashPassword("123456")
	if err != nil {
		return nil, err
	}

	return tx.AdminUser.Create().
		SetUsername("admin").
		SetPasswordHash(password).
		SetIsSuper(true).
		SetCreatedAt(time.Now()).
		Save(ctx)
}

// initSuperAdminRole 初始化超级管理员角色
// 检查是否已存在代码为 "SUPER_ADMIN" 的角色
// 如果存在，直接返回该角色
// 如果不存在，则创建新的超级管理员角色并返回
// 返回创建或获取的角色信息和可能的错误
func initSuperAdminRole(ctx context.Context, tx *ent.Tx) (*ent.AdminRole, error) {
	// 1️⃣ 检查超级管理员角色是否存在
	exist, err := tx.AdminRole.Query().
		Where(adminrole.CodeEQ("SUPER_ADMIN")).
		Exist(ctx)
	if err != nil {
		return nil, err
	}
	if exist {
		return tx.AdminRole.Query().
			Where(adminrole.CodeEQ("SUPER_ADMIN")).
			First(ctx)
	}
	// 2️⃣ 创建超级管理员角色
	role, err := tx.AdminRole.Create().
		SetName("超级管理员").
		SetCode("SUPER_ADMIN").
		SetDataScope(1).
		SetIsSystem(true). // 系统内置，禁止删除
		SetIsActive(true). // 默认启用
		Save(ctx)
	if err != nil {
		return nil, err
	}

	return role, nil
}

// initDefaultPermissions 幂等写入系统内置权限清单
// 清单定义在 permissions.go 的 DefaultPermissions 中：
// Type=1 菜单、Type=2 按钮、Type=3 后端 API（权限中间件据此鉴权）
func initDefaultPermissions(ctx context.Context, tx *ent.Tx) ([]*ent.AdminPermission, error) {
	// 先清理历史遗留的废弃权限，避免脏数据混入权限列表
	if err := CleanupLegacyPermissions(ctx, tx); err != nil {
		return nil, err
	}

	return UpsertPermissions(ctx, tx, DefaultPermissions)
}
