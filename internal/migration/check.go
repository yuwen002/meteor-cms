package migration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"

	"github.com/go-sql-driver/mysql"
	entmigrate "github.com/yuwen002/meteor-cms/ent/migrate"
)

// DatabaseName 从 DSN 解析库名，供 information_schema 查询使用。
func DatabaseName(dsn string) (string, error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return "", fmt.Errorf("解析 MYSQL_DSN 失败: %w", err)
	}
	if cfg.DBName == "" {
		return "", errors.New("MYSQL_DSN 未指定数据库名")
	}
	return cfg.DBName, nil
}

// Drift 描述「期望结构」与「当前库」的差异，供 -check / -dry-run 输出。
// 只检查缺失的表与列；索引以清单形式列出，不自动判断是否缺失，
// 因为 Ent 生成的索引名在不同版本可能变化，误报会掩盖真正的结构问题。
type Drift struct {
	MissingTables   []string
	MissingColumns  []string
	ExpectedIndexes []string
}

func (d Drift) Empty() bool {
	return len(d.MissingTables) == 0 && len(d.MissingColumns) == 0
}

// DetectDrift 只读 information_schema，不执行任何 DDL。
func DetectDrift(ctx context.Context, db *sql.DB, dbName string) (Drift, error) {
	var d Drift
	tables, err := querySet(ctx, db,
		"SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA=?", dbName)
	if err != nil {
		return d, fmt.Errorf("读取现有表失败: %w", err)
	}
	columns, err := queryPairs(ctx, db,
		"SELECT TABLE_NAME, COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=?", dbName)
	if err != nil {
		return d, fmt.Errorf("读取现有列失败: %w", err)
	}

	for _, t := range entmigrate.Tables {
		if !tables[t.Name] {
			d.MissingTables = append(d.MissingTables, t.Name)
			continue
		}
		for _, c := range t.Columns {
			key := t.Name + "." + c.Name
			if !columns[key] {
				d.MissingColumns = append(d.MissingColumns, key)
			}
		}
		for _, i := range t.Indexes {
			names := make([]string, 0, len(i.Columns))
			for _, c := range i.Columns {
				names = append(names, c.Name)
			}
			d.ExpectedIndexes = append(d.ExpectedIndexes,
				fmt.Sprintf("%s(%s)%s", t.Name, joinComma(names), uniqueMark(i.Unique)))
		}
	}
	sort.Strings(d.MissingTables)
	sort.Strings(d.MissingColumns)
	sort.Strings(d.ExpectedIndexes)
	return d, nil
}

func joinComma(items []string) string {
	out := ""
	for i, s := range items {
		if i > 0 {
			out += ","
		}
		out += s
	}
	return out
}

func uniqueMark(unique bool) string {
	if unique {
		return " UNIQUE"
	}
	return ""
}

func querySet(ctx context.Context, db *sql.DB, query string, args ...any) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]bool)
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out[v] = true
	}
	return out, rows.Err()
}

func queryPairs(ctx context.Context, db *sql.DB, query string, args ...any) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]bool)
	for rows.Next() {
		var a, b string
		if err := rows.Scan(&a, &b); err != nil {
			return nil, err
		}
		out[a+"."+b] = true
	}
	return out, rows.Err()
}
