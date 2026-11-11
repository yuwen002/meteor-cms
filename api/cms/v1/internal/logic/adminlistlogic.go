// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"
	"time"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/adminuser"
	"github.com/yuwen002/meteor-cms/ent/department"
	"github.com/yuwen002/meteor-cms/ent/predicate"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/yuwen002/meteor-cms/internal/pkg/datascope"
	"github.com/yuwen002/meteor-cms/internal/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminListLogic {
	return &AdminListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminListLogic) AdminList(req *types.AdminListReq) (resp *types.AdminListResp, err error) {
	offset, pageSize := utils.NormalizePage(req.Page, req.PageSize, utils.DefaultPageSize)
	page := offset/pageSize + 1

	// ent 查询器
	query := l.svcCtx.EntClient.AdminUser.Query()

	// 关键字搜索
	if req.Keyword != "" {
		query = query.Where(
			adminuser.Or(
				adminuser.UsernameContains(req.Keyword),
				adminuser.NicknameContains(req.Keyword),
				adminuser.EmailContains(req.Keyword),
			),
		)
	}

	// 状态过滤
	if req.Active == 1 {
		query = query.Where(adminuser.IsActive(true))
	} else if req.Active == 2 {
		query = query.Where(adminuser.IsActive(false))
	}

	// 数据权限过滤：按当前用户所属角色的数据范围收敛可见管理员
	userID, ok := utils.CurrentUserID(l.ctx)
	if !ok {
		return nil, common.NewBizError(common.ErrMissingUserContext)
	}

	scope, err := datascope.Resolve(l.ctx, l.svcCtx.EntClient, userID)
	if err != nil {
		// 解析失败一律按最严格处理，绝不退回全量
		l.Logger.Errorf("解析数据权限失败，用户ID: %d, 错误: %v", userID, err)
		scope = datascope.BuildFallbackFilter(userID)
	}

	if scope.Restricted {
		// 可见范围为空时直接返回空结果，不查库
		if scope.Empty() {
			return &types.AdminListResp{
				Total:    0,
				Page:     page,
				PageSize: pageSize,
				List:     []types.AdminItem{},
			}, nil
		}

		conds := make([]predicate.AdminUser, 0, 2)
		if len(scope.DeptIDs) > 0 {
			conds = append(conds, adminuser.DeptIDIn(scope.DeptIDs...))
		}
		if scope.OnlySelf {
			conds = append(conds, adminuser.IDEQ(scope.UserID))
		}
		query = query.Where(adminuser.Or(conds...))
	}

	// 按部门筛选：先确认部门仍存在且启用，再与数据可见范围求交，
	// 避免通过构造 dept_id 探测不可见部门，或让已删/停用部门被重新检索出来。
	if req.DeptId > 0 {
		active, e := l.svcCtx.EntClient.Department.
			Query().
			Where(department.IDEQ(req.DeptId), department.IsActiveEQ(true)).
			Exist(l.ctx)
		if e != nil {
			l.Logger.Errorf("校验筛选部门失败: %v", e)
			return nil, common.NewBizError(common.ErrAdminListFailed)
		}
		if !deptFilterAllowed(scope, active, req.DeptId) {
			return &types.AdminListResp{
				Total:    0,
				Page:     page,
				PageSize: pageSize,
				List:     []types.AdminItem{},
			}, nil
		}
		query = query.Where(adminuser.DeptIDEQ(req.DeptId))
	}

	// 总数
	total, err := query.Count(l.ctx)
	if err != nil {
		l.Logger.Errorf("获取管理员总数失败: %v", err)
		return nil, common.NewBizError(common.ErrAdminCountFailed)
	}

	// 分页数据
	list, err := query.
		Order(ent.Desc(adminuser.FieldCreatedAt)).
		Offset(offset).
		Limit(pageSize).
		All(l.ctx)
	if err != nil {
		l.Logger.Errorf("获取管理员列表失败: %v", err)
		return nil, common.NewBizError(common.ErrAdminListFailed)
	}

	// 一次性取部门名用于列表展示，避免逐行 N+1 查询；失败只记日志，不影响列表主体
	deptNames := make(map[int64]string)
	if len(list) > 0 {
		depts, e := l.svcCtx.EntClient.Department.Query().All(l.ctx)
		if e != nil {
			l.Logger.Errorf("查询部门名称失败: %v", e)
		} else {
			for _, d := range depts {
				deptNames[d.ID] = d.Name
			}
		}
	}

	// 转换结构
	items := make([]types.AdminItem, 0, len(list))
	for _, u := range list {
		item := types.AdminItem{
			Id:        u.ID,
			Username:  u.Username,
			Nickname:  u.Nickname,
			Email:     u.Email,
			Phone:     u.Phone,
			Avatar:    u.Avatar,
			IsSuper:   u.IsSuper,
			IsActive:  u.IsActive,
			CreatedAt: u.CreatedAt.Format(time.RFC3339),
		}
		if u.LastLoginAt != nil {
			item.LastLoginAt = u.LastLoginAt.Format(time.RFC3339)
		}
		if u.DeptID != nil {
			item.DeptId = *u.DeptID
			item.DeptName = deptNames[item.DeptId]
		}
		items = append(items, item)
	}

	// 返回带上 Page 和 PageSize
	return &types.AdminListResp{
		Total:    int64(total),
		Page:     page,
		PageSize: pageSize,
		List:     items,
	}, nil
}

// deptFilterAllowed 判定「按指定部门筛选」是否落在当前可见范围内。
// 抽成纯函数便于脱离数据库做防越权回归测试。
func deptFilterAllowed(scope *datascope.Filter, deptActive bool, deptID int64) bool {
	if deptID <= 0 || !deptActive {
		return false
	}
	return !scope.Restricted || scope.AllowsDept(deptID)
}
