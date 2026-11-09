package common

const (
	// Success 成功
	Success = 0

	// Common errors (1000-1999)
	ErrInternalServer = 1000 // 服务器内部错误

	// Auth errors (2000-2999)
	ErrUnauthorized       = 2001 // 未授权（缺少认证凭证）
	ErrForbidden          = 2003 // 禁止访问
	ErrPermissionDenied   = 2002 // 权限不足
	ErrMissingUserContext = 2004 // 用户上下文信息缺失
	ErrMissingCaptcha     = 2005 // 验证码不能为空
	ErrInvalidCaptcha     = 2006 // 验证码错误
	ErrAccountInactive    = 2007 // 账号未激活
	ErrCaptchaGenerate    = 2008 // 验证码生成失败
	ErrInvalidToken       = 2009 // 无效的token
	ErrLoginFailed        = 2010 // 登录失败（用户名或密码错误）
	ErrAccountLocked      = 2011 // 账号因多次登录失败被临时锁定
	ErrTokenRevoked       = 2012 // token 已被吊销（账号被踢下线/改密后旧令牌失效）

	// Admin user errors (3000-3999)
	ErrAdminUserAlreadyExists  = 3001 // 管理员已存在
	ErrAdminUserNotFound       = 3002 // 管理员不存在
	ErrAdminPasswordIncorrect  = 3003 // 密码错误
	ErrAdminCountFailed        = 3004 // 获取管理员总数失败
	ErrAdminListFailed         = 3005 // 获取管理员列表失败
	ErrAdminPasswordHashFail   = 3006 // 密码加密失败
	ErrAdminCreateFailed       = 3007 // 创建管理员失败
	ErrAdminEmailAlreadyExists = 3008 // 邮箱已被注册
	ErrUserIDFormat            = 3009 // 用户ID格式错误
	ErrPasswordUpdateFailed    = 3010 // 密码更新失败
	ErrCannotResetOwnPassword  = 3011 // 不能重置自己的密码
	ErrResetTokenInvalid       = 3012 // 密码重置令牌无效
	ErrResetTokenExpired       = 3013 // 密码重置令牌已过期
	ErrMailSendFailed          = 3014 // 邮件发送失败

	// Request validation errors (4000-4999)
	ErrInvalidParams = 4001 // 参数错误
	ErrFileNotFound  = 4002 // 文件不存在

	// Department errors (5000-5199)
	ErrDepartmentNotFound        = 5001 // 部门不存在
	ErrDepartmentCreateFail      = 5002 // 创建部门失败
	ErrDepartmentUpdateFail      = 5003 // 更新部门失败
	ErrDepartmentDeleteFail      = 5004 // 删除部门失败
	ErrDepartmentListFail        = 5005 // 获取部门列表失败
	ErrDepartmentParentNotExist  = 5006 // 父级部门不存在
	ErrDepartmentSetSelfAsParent = 5007 // 不能将自己设为父级部门
	ErrDepartmentHasChildren     = 5008 // 该部门下存在子部门，禁止删除
	ErrDepartmentSetLeaderFail   = 5009 // 设置部门负责人失败
	ErrDepartmentNameExists      = 5010 // 部门名称已存在
	ErrDepartmentParentNotActive = 5011 // 父部门未启用

	// Role errors (5200-5399)
	ErrRoleNotFound          = 5201 // 角色不存在
	ErrRoleCreateFail        = 5202 // 创建角色失败
	ErrRoleUpdateFail        = 5203 // 更新角色失败
	ErrRoleDeleteFail        = 5204 // 删除角色失败
	ErrRoleListFail          = 5205 // 获取角色列表失败
	ErrRoleCodeExists        = 5206 // 角色编码已存在
	ErrRoleNameExists        = 5207 // 角色名称已存在
	ErrRoleIsSystemRole      = 5208 // 系统内置角色，禁止修改
	ErrRoleIsInUse           = 5209 // 角色正在使用中，禁止删除
	ErrRolePermissionDenied  = 5210 // 没有操作该角色的权限
	ErrRoleCodeCannotBeEmpty = 5211 // 角色编码不能为空
	ErrRoleNameCannotBeEmpty = 5212 // 角色名称不能为空

	// Permission errors (5400-5599)
	ErrPermissionNotFound          = 5401 // 权限不存在
	ErrPermissionCreateFail        = 5402 // 创建权限失败
	ErrPermissionUpdateFail        = 5403 // 更新权限失败
	ErrPermissionDeleteFail        = 5404 // 删除权限失败
	ErrPermissionListFail          = 5405 // 获取权限列表失败
	ErrPermissionExists            = 5406 // 权限标识已存在
	ErrPermissionNameExists        = 5407 // 权限名称已存在
	ErrParentPermissionNotFound    = 5408 // 父级权限不存在
	ErrPermissionIsSystem          = 5409 // 系统内置权限，禁止修改
	ErrPermissionInUse             = 5410 // 权限正在使用中，禁止删除
	ErrPermissionNameCannotBeEmpty = 5411 // 权限名称不能为空
	ErrPermissionTypeInvalid       = 5412 // 权限类型无效

	// Dictionary errors (5600-5699)
	ErrDictNotFound     = 5601 // 字典不存在
	ErrDictCodeExists   = 5602 // 字典编码已存在
	ErrDictCreateFail   = 5603 // 创建字典失败
	ErrDictUpdateFail   = 5604 // 更新字典失败
	ErrDictDeleteFail   = 5605 // 删除字典失败
	ErrDictItemNotFound = 5606 // 字典项不存在
	ErrDictItemExists   = 5607 // 同一字典下该字典项已存在

	// System config errors (5700-5799)
	ErrConfigNotFound     = 5701 // 配置不存在
	ErrConfigKeyExists    = 5702 // 配置键已存在
	ErrConfigCreateFail   = 5703 // 创建配置失败
	ErrConfigUpdateFail   = 5704 // 更新配置失败
	ErrConfigDeleteSystem = 5705 // 系统内置配置禁止删除

	// Article errors (5800-5899)
	ErrArticleNotFound        = 5801 // 文章不存在
	ErrArticleSlugExists      = 5802 // 文章别名已被占用
	ErrArticleCreateFail      = 5803 // 创建文章失败
	ErrArticleUpdateFail      = 5804 // 更新文章失败
	ErrArticleDeleteFail      = 5805 // 删除文章失败
	ErrArticleListFail        = 5806 // 获取文章列表失败
	ErrArticleInvalidSlug     = 5807 // 文章别名格式不合法
	ErrArticlePublishFail     = 5808 // 发布文章失败
	ErrArticleWithdrawFail    = 5809 // 撤回文章失败
	ErrArticleCategoryInvalid = 5810 // 所属分类不存在或已禁用
	ErrArticleTagInvalid      = 5811 // 标签不存在或已禁用
	ErrArticleInvalidTime     = 5812 // 定时发布时间格式不合法
	ErrArticleSetTopFail      = 5813 // 设置置顶失败

	// Category & Tag errors (5900-5999)
	ErrCategoryNotFound     = 5901 // 分类不存在
	ErrCategoryNameExists   = 5902 // 分类名称已存在
	ErrCategorySlugExists   = 5903 // 分类别名已被占用
	ErrCategoryHasChildren  = 5904 // 该分类下存在子分类
	ErrCategoryHasArticles  = 5905 // 该分类下存在文章
	ErrCategoryCreateFail   = 5906 // 创建分类失败
	ErrCategoryUpdateFail   = 5907 // 更新分类失败
	ErrCategoryDeleteFail   = 5908 // 删除分类失败
	ErrCategoryParentSelf   = 5909 // 不能将自己设为父级分类
	ErrCategoryParentNested = 5910 // 不能将子分类设为父级分类
	ErrTagNotFound          = 5921 // 标签不存在
	ErrTagNameExists        = 5922 // 标签名称已存在
	ErrTagSlugExists        = 5923 // 标签别名已被占用
	ErrTagCreateFail        = 5924 // 创建标签失败
	ErrTagUpdateFail        = 5925 // 更新标签失败
	ErrTagDeleteFail        = 5926 // 删除标签失败

	// Trash / revision / review errors (6000-6099)
	ErrTrashInvalidType     = 6001 // 回收站实体类型不合法
	ErrTrashNotFound        = 6002 // 回收站记录不存在
	ErrTrashRestoreFail     = 6003 // 恢复失败
	ErrTrashPurgeFail       = 6004 // 彻底删除失败
	ErrTrashSlugConflict    = 6005 // 原 slug 已被占用，恢复后需重新设置
	ErrRevisionNotFound     = 6021 // 修订版本不存在
	ErrRevisionListFail     = 6022 // 获取修订历史失败
	ErrRevisionRestoreFail  = 6023 // 恢复修订版本失败
	ErrArticleForbidden     = 6031 // 无权操作他人文章
	ErrArticleApproveNeeded = 6032 // 待审核文章需审核通过后才能发布
	ErrArticleNotPending    = 6033 // 文章不处于待审核状态
	ErrArticleApproveForbid = 6034 // 缺少审核权限
	ErrArticlePublishForbid = 6035 // 缺少发布权限
	ErrArticleInvalidEntity = 6036 // 内容实体类型不合法
	ErrArticleReadForbidden = 6037 // 无权查看他人文章
)

var errorMessages = map[int]string{
	Success:                    "success",
	ErrInternalServer:          "服务器内部错误",
	ErrUnauthorized:            "请先登录",
	ErrPermissionDenied:        "权限不足",
	ErrForbidden:               "禁止访问",
	ErrMissingUserContext:      "用户会话已过期，请重新登录",
	ErrInvalidToken:            "无效的token",
	ErrLoginFailed:             "用户名或密码错误",
	ErrAccountLocked:           "账号已被临时锁定，请稍后再试",
	ErrTokenRevoked:            "登录状态已失效，请重新登录",
	ErrMissingCaptcha:          "验证码不能为空",
	ErrInvalidCaptcha:          "验证码错误",
	ErrAccountInactive:         "账号未激活，请联系管理员",
	ErrCaptchaGenerate:         "验证码生成失败，请重试",
	ErrAdminUserAlreadyExists:  "管理员已存在",
	ErrAdminUserNotFound:       "管理员不存在",
	ErrAdminPasswordIncorrect:  "密码错误",
	ErrAdminCountFailed:        "获取管理员总数失败",
	ErrAdminListFailed:         "获取管理员列表失败",
	ErrAdminPasswordHashFail:   "密码加密失败",
	ErrAdminCreateFailed:       "创建管理员失败",
	ErrAdminEmailAlreadyExists: "邮箱已被注册",
	ErrUserIDFormat:            "用户ID格式错误",
	ErrPasswordUpdateFailed:    "密码更新失败",
	ErrCannotResetOwnPassword:  "不能重置自己的密码，请使用修改密码功能",
	ErrResetTokenInvalid:       "重置链接无效",
	ErrResetTokenExpired:       "重置链接已过期，请重新申请",
	ErrMailSendFailed:          "邮件发送失败，请稍后重试",
	ErrInvalidParams:           "参数错误",
	ErrFileNotFound:            "文件不存在",
	// Department error messages
	ErrDepartmentNotFound:        "部门不存在",
	ErrDepartmentCreateFail:      "创建部门失败",
	ErrDepartmentUpdateFail:      "更新部门失败",
	ErrDepartmentDeleteFail:      "删除部门失败",
	ErrDepartmentListFail:        "获取部门列表失败",
	ErrDepartmentParentNotExist:  "父级部门不存在",
	ErrDepartmentSetSelfAsParent: "不能将自己设为父级部门",
	ErrDepartmentHasChildren:     "该部门下存在子部门，禁止删除",
	ErrDepartmentSetLeaderFail:   "设置部门负责人失败",
	ErrDepartmentNameExists:      "部门名称已存在",
	ErrDepartmentParentNotActive: "父部门未启用，无法添加子部门",
	// Role error messages
	ErrRoleNotFound:          "角色不存在",
	ErrRoleCreateFail:        "创建角色失败",
	ErrRoleUpdateFail:        "更新角色失败",
	ErrRoleDeleteFail:        "删除角色失败",
	ErrRoleListFail:          "获取角色列表失败",
	ErrRoleCodeExists:        "角色编码已存在",
	ErrRoleNameExists:        "角色名称已存在",
	ErrRoleIsSystemRole:      "系统内置角色，禁止修改",
	ErrRoleIsInUse:           "角色正在使用中，禁止删除",
	ErrRolePermissionDenied:  "没有操作该角色的权限",
	ErrRoleCodeCannotBeEmpty: "角色编码不能为空",
	ErrRoleNameCannotBeEmpty: "角色名称不能为空",
	// Permission error messages
	ErrPermissionNotFound:          "权限不存在",
	ErrPermissionCreateFail:        "创建权限失败",
	ErrPermissionUpdateFail:        "更新权限失败",
	ErrPermissionDeleteFail:        "删除权限失败",
	ErrPermissionListFail:          "获取权限列表失败",
	ErrPermissionExists:            "权限标识已存在",
	ErrPermissionNameExists:        "权限名称已存在",
	ErrParentPermissionNotFound:    "父级权限不存在",
	ErrPermissionIsSystem:          "系统内置权限，禁止修改",
	ErrPermissionInUse:             "权限正在使用中，禁止删除",
	ErrPermissionNameCannotBeEmpty: "权限名称不能为空",
	ErrPermissionTypeInvalid:       "权限类型无效",
	// Dictionary error messages
	ErrDictNotFound:     "字典不存在",
	ErrDictCodeExists:   "字典编码已存在",
	ErrDictCreateFail:   "创建字典失败",
	ErrDictUpdateFail:   "更新字典失败",
	ErrDictDeleteFail:   "删除字典失败",
	ErrDictItemNotFound: "字典项不存在",
	ErrDictItemExists:   "该字典下已存在相同的字典项",
	// System config error messages
	ErrConfigNotFound:     "配置不存在",
	ErrConfigKeyExists:    "配置键已存在",
	ErrConfigCreateFail:   "创建配置失败",
	ErrConfigUpdateFail:   "更新配置失败",
	ErrConfigDeleteSystem: "系统内置配置禁止删除",
	// Article error messages
	ErrArticleNotFound:        "文章不存在",
	ErrArticleSlugExists:      "文章别名已被占用，请换一个",
	ErrArticleCreateFail:      "创建文章失败",
	ErrArticleUpdateFail:      "更新文章失败",
	ErrArticleDeleteFail:      "删除文章失败",
	ErrArticleListFail:        "获取文章列表失败",
	ErrArticleInvalidSlug:     "文章别名只能包含小写字母、数字和连字符",
	ErrArticlePublishFail:     "发布文章失败",
	ErrArticleWithdrawFail:    "撤回文章失败",
	ErrArticleCategoryInvalid: "所属分类不存在或已禁用",
	ErrArticleTagInvalid:      "所选标签不存在或已禁用",
	ErrArticleInvalidTime:     "定时发布时间格式不合法",
	ErrArticleSetTopFail:      "设置置顶失败",
	// Category & Tag error messages
	ErrCategoryNotFound:     "分类不存在",
	ErrCategoryNameExists:   "分类名称已存在",
	ErrCategorySlugExists:   "分类别名已被占用",
	ErrCategoryHasChildren:  "该分类下存在子分类，禁止删除",
	ErrCategoryHasArticles:  "该分类下存在文章，禁止删除",
	ErrCategoryCreateFail:   "创建分类失败",
	ErrCategoryUpdateFail:   "更新分类失败",
	ErrCategoryDeleteFail:   "删除分类失败",
	ErrCategoryParentSelf:   "不能将自己设为父级分类",
	ErrCategoryParentNested: "不能将子分类设为父级分类",
	ErrTagNotFound:          "标签不存在",
	ErrTagNameExists:        "标签名称已存在",
	ErrTagSlugExists:        "标签别名已被占用",
	ErrTagCreateFail:        "创建标签失败",
	ErrTagUpdateFail:        "更新标签失败",
	ErrTagDeleteFail:        "删除标签失败",
	// Trash / revision / review error messages
	ErrTrashInvalidType:     "回收站实体类型不合法",
	ErrTrashNotFound:        "回收站记录不存在",
	ErrTrashRestoreFail:     "恢复失败",
	ErrTrashPurgeFail:       "彻底删除失败",
	ErrTrashSlugConflict:    "原别名已被其它内容占用，请先修改后再恢复",
	ErrRevisionNotFound:     "修订版本不存在",
	ErrRevisionListFail:     "获取修订历史失败",
	ErrRevisionRestoreFail:  "恢复修订版本失败",
	ErrArticleForbidden:     "只能操作自己创建的文章",
	ErrArticleApproveNeeded: "待审核文章需审核通过后才能发布",
	ErrArticleNotPending:    "文章不处于待审核状态",
	ErrArticleApproveForbid: "缺少文章审核权限",
	ErrArticlePublishForbid: "缺少文章发布权限",
	ErrArticleInvalidEntity: "内容实体类型不合法",
	ErrArticleReadForbidden: "无权查看他人文章",
}

// GetErrorMessage 获取错误码对应的错误信息
// 参数：
//   - code: 错误码
//
// 返回值：
//   - string: 错误信息，如果错误码不存在则返回"未知错误"
func GetErrorMessage(code int) string {
	if msg, ok := errorMessages[code]; ok {
		return msg
	}
	return "未知错误"
}
