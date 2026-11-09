package migration

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

// DedupePermissions 在加 (type, permission, method, api_path) 唯一索引之前清理历史重复行。
//
// 旧版本 UpsertPermissions 只做「先查后写」，表上又没有唯一约束，并发迁移可能写入重复
// 权限；权限中间件遇到重复 (method, api_path) 会判定歧义并拒绝请求。这里保留每组最小 id，
// 并把角色绑定迁移过去（撞唯一键的重复绑定由 UPDATE IGNORE 跳过、随后清理）。
func DedupePermissions(ctx context.Context, db *sql.DB) (int64, error) {
	permExists, err := tableExists(ctx, db, "admin_permissions")
	if err != nil || !permExists {
		return 0, err
	}
	roleExists, err := tableExists(ctx, db, "admin_role_permissions")
	if err != nil {
		return 0, err
	}

	rows, err := db.QueryContext(ctx, `
		SELECT GROUP_CONCAT(id ORDER BY id ASC) AS ids
		FROM admin_permissions
		GROUP BY type, permission, method, api_path
		HAVING COUNT(*) > 1`)
	if err != nil {
		return 0, fmt.Errorf("查询重复权限失败: %w", err)
	}
	type group struct {
		keep   int64
		others []int64
	}
	var groups []group
	for rows.Next() {
		var csv string
		if err := rows.Scan(&csv); err != nil {
			_ = rows.Close()
			return 0, err
		}
		var ids []int64
		for _, part := range strings.Split(csv, ",") {
			id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
			if err != nil {
				_ = rows.Close()
				return 0, fmt.Errorf("解析权限 ID %q 失败: %w", part, err)
			}
			ids = append(ids, id)
		}
		if len(ids) < 2 {
			continue
		}
		groups = append(groups, group{keep: ids[0], others: ids[1:]})
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, err
	}
	_ = rows.Close()
	if len(groups) == 0 {
		return 0, nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	var removed int64
	for _, g := range groups {
		in, args := placeholders(g.others)
		if roleExists {
			for _, oid := range g.others {
				if _, err := tx.ExecContext(ctx,
					"UPDATE IGNORE admin_role_permissions SET permission_id=? WHERE permission_id=?",
					g.keep, oid); err != nil {
					return 0, fmt.Errorf("迁移角色权限绑定失败: %w", err)
				}
			}
			if _, err := tx.ExecContext(ctx,
				"DELETE FROM admin_role_permissions WHERE permission_id IN ("+in+")", args...); err != nil {
				return 0, fmt.Errorf("清理重复角色权限绑定失败: %w", err)
			}
		}
		// 子菜单可能引用了将被删除的重复菜单；外键为 SET NULL，这里显式兜底旧库无外键的情况。
		if _, err := tx.ExecContext(ctx,
			"UPDATE admin_permissions SET parent_id=NULL WHERE parent_id IN ("+in+")", args...); err != nil {
			return 0, fmt.Errorf("清理父级引用失败: %w", err)
		}
		res, err := tx.ExecContext(ctx,
			"DELETE FROM admin_permissions WHERE id IN ("+in+")", args...)
		if err != nil {
			return 0, fmt.Errorf("删除重复权限失败: %w", err)
		}
		n, err := res.RowsAffected()
		if err == nil {
			removed += n
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return removed, nil
}

func tableExists(ctx context.Context, db *sql.DB, name string) (bool, error) {
	var count int
	if err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=?",
		name).Scan(&count); err != nil {
		return false, fmt.Errorf("检查表 %s 是否存在失败: %w", name, err)
	}
	return count > 0, nil
}

func placeholders(ids []int64) (string, []any) {
	parts := make([]string, 0, len(ids))
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, "?")
		args = append(args, id)
	}
	return strings.Join(parts, ","), args
}
