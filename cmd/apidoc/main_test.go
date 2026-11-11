package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// 内容模块路由由 contentroutes.go 手工注册，必须出现在 gen-doc 输出里，
// 否则 openapi.json 会漏掉文章发布相关的全部接口。
func TestGenerateIncludesContentModule(t *testing.T) {
	spec := generate(loadAPI("../../api/cms/v1/cms.api"))
	paths, ok := spec["paths"].(map[string]interface{})
	if !ok {
		t.Fatal("spec.paths 类型错误")
	}

	want := []string{
		"/admin/articles",
		"/admin/articles/{id}/revisions/{rid}",
		"/admin/articles/{id}/revisions/{rid}/restore",
		"/admin/trash/{type}/{id}/restore",
		"/api/articles",
		"/api/articles/{slug}",
		"/api/taxonomy",
		"/api/rss",
		"/sitemap.xml",
		"/robots.txt",
		"/admin/articles/batch-offline",
		"/admin/articles/batch-delete",
		"/healthz",
	}
	for _, p := range want {
		if _, ok := paths[p]; !ok {
			t.Errorf("openapi 缺少内容模块路径：%s", p)
		}
	}
	for p := range paths {
		if strings.Contains(p, ":") {
			t.Errorf("路径参数未转换为 OpenAPI 占位符：%s", p)
		}
	}

	components, ok := spec["components"].(map[string]interface{})
	if !ok {
		t.Fatal("spec.components 类型错误")
	}
	schemas, ok := components["schemas"].(map[string]interface{})
	if !ok {
		t.Fatal("spec.components.schemas 类型错误")
	}
	for _, name := range []string{"ArticleDetailResp", "PublicArticleDetailResp", "TrashListResp"} {
		if _, ok := schemas[name]; !ok {
			t.Errorf("openapi 缺少 schema：%s", name)
		}
	}

	// GET /admin/articles 的分页/筛选条件是 query 参数，不应被当成请求体
	listOp := paths["/admin/articles"].(map[string]interface{})["get"].(map[string]interface{})
	params, ok := listOp["parameters"].([]interface{})
	if !ok || len(params) == 0 {
		t.Error("GET /admin/articles 缺少 query 参数定义")
	}
	if _, ok := listOp["requestBody"]; ok {
		t.Error("GET /admin/articles 不应生成 requestBody")
	}

	// RSS 是纯文本输出，不能声明成 application/json
	rssOp := paths["/api/rss"].(map[string]interface{})["get"].(map[string]interface{})
	responses := rssOp["responses"].(map[string]interface{})["200"].(map[string]interface{})
	content := responses["content"].(map[string]interface{})
	if _, ok := content["application/rss+xml"]; !ok {
		t.Error("/api/rss 响应缺少 application/rss+xml")
	}

	// 没有请求体的 GET 也要带上响应模型引用
	taxOp := paths["/api/taxonomy"].(map[string]interface{})["get"].(map[string]interface{})
	taxResp := taxOp["responses"].(map[string]interface{})["200"].(map[string]interface{})
	taxSchema := taxResp["content"].(map[string]interface{})["application/json"].(map[string]interface{})["schema"].(map[string]interface{})
	if taxSchema["$ref"] != "#/components/schemas/PublicTaxonomyResp" {
		t.Errorf("/api/taxonomy 响应模型错误：%v", taxSchema)
	}
}

// TestGenerateIncludesAllRegisteredRoutes 以 Go 路由注册为基准做覆盖校验。
// 自动扫描所有 *routes.go，避免新增路由注册文件时忘记把它加进手工清单。
func TestGenerateIncludesAllRegisteredRoutes(t *testing.T) {
	spec := generate(loadAPI("../../api/cms/v1/cms.api"))
	paths, ok := spec["paths"].(map[string]interface{})
	if !ok {
		t.Fatal("spec.paths 类型错误")
	}

	re := regexp.MustCompile(`Method:\s*http\.Method(\w+),\s*Path:\s*"([^"]+)"`)
	handlerDir := "../../api/cms/v1/internal/handler"
	files, err := filepath.Glob(filepath.Join(handlerDir, "*routes.go"))
	if err != nil {
		t.Fatalf("扫描路由文件失败: %v", err)
	}
	if len(files) < 5 {
		t.Fatalf("扫描到的路由文件过少（%d）: %v", len(files), files)
	}
	total := 0
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("读取路由文件失败 %s: %v", file, err)
		}
		for _, m := range re.FindAllStringSubmatch(string(b), -1) {
			total++
			method := strings.ToLower(m[1])
			path := pathParamRe.ReplaceAllString(m[2], "{$1}")
			ops, ok := paths[path].(map[string]interface{})
			if !ok {
				t.Errorf("文档缺少路由 %s %s（%s）", method, path, file)
				continue
			}
			if _, ok := ops[method]; !ok {
				t.Errorf("文档缺少操作 %s %s（%s）", method, path, file)
			}
		}
	}
	if total < 80 {
		t.Errorf("解析到的 Go 路由数量异常: %d", total)
	}
}

func collectSchemaRefs(v interface{}, out map[string]bool) {
	switch x := v.(type) {
	case map[string]interface{}:
		if ref, ok := x["$ref"].(string); ok {
			out[strings.TrimPrefix(ref, "#/components/schemas/")] = true
		}
		for _, val := range x {
			collectSchemaRefs(val, out)
		}
	case []interface{}:
		for _, val := range x {
			collectSchemaRefs(val, out)
		}
	}
}

func schemasForTest(t *testing.T) (map[string]interface{}, map[string]interface{}) {
	t.Helper()
	spec := generate(loadAPI("../../api/cms/v1/cms.api"))
	paths, ok := spec["paths"].(map[string]interface{})
	if !ok {
		t.Fatal("spec.paths 类型错误")
	}
	components, ok := spec["components"].(map[string]interface{})
	if !ok {
		t.Fatal("spec.components 类型错误")
	}
	schemas, ok := components["schemas"].(map[string]interface{})
	if !ok {
		t.Fatal("spec.components.schemas 类型错误")
	}
	return paths, schemas
}

// TestAllRefsResolve 递归校验所有 $ref 都能命中 components.schemas。
// 之前 cms.api 尾部的独立 type Xxx {} 没被解析，产物里留下 5 处悬空引用。
func TestAllRefsResolve(t *testing.T) {
	spec := generate(loadAPI("../../api/cms/v1/cms.api"))
	components := spec["components"].(map[string]interface{})
	schemas := components["schemas"].(map[string]interface{})
	refs := map[string]bool{}
	collectSchemaRefs(spec, refs)
	if len(refs) < 20 {
		t.Fatalf("解析到的 $ref 数量异常: %d", len(refs))
	}
	for name := range refs {
		if _, ok := schemas[name]; !ok {
			t.Errorf("悬空 $ref：#/components/schemas/%s", name)
		}
	}
}

// TestEmbeddedAndStandaloneSchemasHaveProperties 覆盖两类字段解析盲区：
// 匿名嵌入字段需要平铺，独立 type {} 需要被 parseTypes 收进 schemas。
// 注意：纯 path/form 参数类型是合理的空体，不能全量断言。
func TestEmbeddedAndStandaloneSchemasHaveProperties(t *testing.T) {
	_, schemas := schemasForTest(t)
	for _, name := range []string{
		"RoleDetailResp", "PermissionDetailResp",
		"DashboardStatsResp", "FileItem", "FileListReq", "FileListResp", "FileDeleteReq", "FileRenameReq",
	} {
		sch, ok := schemas[name].(map[string]interface{})
		if !ok {
			t.Errorf("缺少 schema：%s", name)
			continue
		}
		props, ok := sch["properties"].(map[string]interface{})
		if !ok || len(props) == 0 {
			t.Errorf("schema %s 的 properties 为空", name)
		}
	}
}

// TestArrayResponses returns ([]T) 必须生成 array + items.$ref，不能被降级成 object。
func TestArrayResponses(t *testing.T) {
	paths, _ := schemasForTest(t)
	for _, tc := range []struct{ path, itemRef string }{
		{"/admin/departments/tree", "#/components/schemas/DepartmentTreeResp"},
		{"/admin/permissions/tree", "#/components/schemas/PermissionTreeItem"},
	} {
		ops, ok := paths[tc.path].(map[string]interface{})
		if !ok {
			t.Errorf("缺少路径 %s", tc.path)
			continue
		}
		op, ok := ops["get"].(map[string]interface{})
		if !ok {
			t.Errorf("缺少操作 get %s", tc.path)
			continue
		}
		responses := op["responses"].(map[string]interface{})
		resp := responses["200"].(map[string]interface{})
		content := resp["content"].(map[string]interface{})
		jsonContent := content["application/json"].(map[string]interface{})
		schema := jsonContent["schema"].(map[string]interface{})
		if schema["type"] != "array" {
			t.Errorf("%s 响应应为 array，实际 %v", tc.path, schema)
			continue
		}
		items, ok := schema["items"].(map[string]interface{})
		if !ok || items["$ref"] != tc.itemRef {
			t.Errorf("%s 的 items 应为 %s，实际 %v", tc.path, tc.itemRef, schema["items"])
		}
	}
}
