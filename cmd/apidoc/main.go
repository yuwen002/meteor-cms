// Command apidoc 解析 Go-Zero 的 cms.api 文件，生成 OpenAPI 3.0.3 文档（openapi.json）。
// 用法：go run ./cmd/apidoc  （在项目根目录执行，输出 openapi.json）
//
// 由于 goctl 1.10.2 不提供 openapi 子命令，这里用零依赖的方式从 .api 源文件直接生成，
// 保证文档与接口定义同源、可复现（make gen-doc）。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type field struct {
	name      string
	json      string
	goType    string
	optional  bool
	paramIn   string // "query" / "path"，来自 form / path 标签
	paramName string
}

// 站点级纯文本输出不走统一 JSON 响应封装
var textResponseTypes = map[string]string{
	"publicRss":     "application/rss+xml",
	"publicSitemap": "application/xml",
	"publicRobots":  "text/plain",
	"publicMetrics": "text/plain",
	"logExport":     "text/csv",
}

var pathParamRe = regexp.MustCompile(`:([A-Za-z_][A-Za-z0-9_]*)`)

type typeDef struct {
	name     string
	fields   []field
	embedded []string // 匿名嵌入的类型名，字段需平铺到外层
}

type route struct {
	handler string
	method  string
	path    string
	req     string
	resp    string
}

func main() {
	src := loadAPI("api/cms/v1/cms.api")
	spec := generate(src)

	b, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "marshal:", err)
		os.Exit(1)
	}
	if err := os.WriteFile("openapi.json", b, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "write:", err)
		os.Exit(1)
	}
	fmt.Println("openapi.json generated")
}

// loadAPI 读取主 api 文件及其 import 的其它 .api 文件（go-zero 支持 import 拆分定义）。
func loadAPI(mainPath string) string {
	read := func(p string) string {
		b, err := os.ReadFile(p)
		if err != nil {
			fmt.Fprintln(os.Stderr, "read", p, err)
			return ""
		}
		return string(b)
	}
	base := read(mainPath)
	dir := filepath.Dir(mainPath)
	reImp := regexp.MustCompile(`import\s*\(([\s\S]*?)\)`)
	for _, m := range reImp.FindAllStringSubmatch(base, -1) {
		for _, line := range strings.Split(m[1], "\n") {
			line = strings.TrimSpace(line)
			line = strings.Trim(line, `"`)
			if line == "" || !strings.HasSuffix(line, ".api") {
				continue
			}
			base += "\n" + read(filepath.Join(dir, line))
		}
	}

	// 手工注册的路由分两份文档源：内容模块（contentroutes.go）与系统模块
	// （routes.go 中的字典/配置/强制下线），都没有进 cms.api。
	for _, name := range []string{"content.doc.api", "system.doc.api", "log.doc.api", "site.doc.api", "ops.doc.api"} {
		docExtra := filepath.Join(dir, name)
		if b, err := os.ReadFile(docExtra); err == nil {
			base += "\n" + string(b)
		} else {
			fmt.Fprintln(os.Stderr, "warn: doc source missing:", name, err)
		}
	}
	return base
}

func stripComments(s string) string {
	reBlock := regexp.MustCompile(`/\*[\s\S]*?\*/`)
	s = reBlock.ReplaceAllString(s, "")
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if idx := strings.Index(l, "//"); idx >= 0 {
			lines[i] = l[:idx]
		}
	}
	return strings.Join(lines, "\n")
}

// findBlock 找到 token 之后的开闭括号（openB/closeB）之间的内容，可匹配多层。
func findBlock(src, token string, openB, closeB byte) []string {
	var out []string
	rest := src
	for {
		idx := strings.Index(rest, token)
		if idx < 0 {
			break
		}
		start := idx + len(token)
		// token 自身以开括号结尾时（如 "type ("），回退到该括号，
		// 否则会跳过当前块、误把下一个括号当成块起点。
		if strings.HasSuffix(token, string(openB)) {
			start--
		}
		k := start
		for k < len(rest) && rest[k] != openB {
			k++
		}
		if k >= len(rest) {
			break
		}
		depth := 0
		j := k
		for j < len(rest) {
			switch rest[j] {
			case openB:
				depth++
			case closeB:
				depth--
				if depth == 0 {
					out = append(out, rest[k+1:j])
					rest = rest[j+1:]
					goto done
				}
			}
			j++
		}
		break
	done:
	}
	return out
}

func splitStructs(block string) []struct {
	name string
	body string
} {
	var res []struct {
		name string
		body string
	}
	rest := block
	for {
		idx := strings.Index(rest, "{")
		if idx < 0 {
			break
		}
		before := strings.TrimSpace(rest[:idx])
		name := before
		// .api 文件用 tab / 换行缩进，不能只按空格取最后一个 token
		if i := strings.LastIndexAny(before, " \t\n"); i >= 0 {
			name = before[i+1:]
		}
		depth := 0
		j := idx
		for j < len(rest) {
			switch rest[j] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					res = append(res, struct {
						name string
						body string
					}{name, rest[idx+1 : j]})
					rest = rest[j+1:]
					goto nxt
				}
			}
			j++
		}
		break
	nxt:
	}
	return res
}

func parseTypes(src string) map[string]typeDef {
	types := map[string]typeDef{}
	fieldRe := regexp.MustCompile(`(\w+)\s+([\w.*\[\]]+)\s+` + "`" + `([^` + "`" + `]*)` + "`")
	embedRe := regexp.MustCompile(`^(\w+)$`)

	parseStruct := func(name, body string) typeDef {
		td := typeDef{name: name}
		for _, line := range strings.Split(body, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			// go-zero 的匿名嵌入字段（如 RoleDetailResp 里的 RoleItem）没有类型与 tag，
			// 语义是把被嵌入类型的字段平铺到外层，而不是生成一个同名属性。
			if em := embedRe.FindStringSubmatch(line); em != nil {
				td.embedded = append(td.embedded, em[1])
				continue
			}
			m := fieldRe.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			f := field{name: m[1], goType: m[2]}
			tag := m[3]
			if jm := regexp.MustCompile(`json:"([^"]*)"`).FindStringSubmatch(tag); jm != nil {
				parts := strings.Split(jm[1], ",")
				f.json = parts[0]
				for _, p := range parts[1:] {
					if p == "optional" {
						f.optional = true
					}
				}
			}
			if fm := regexp.MustCompile(`form:"([^"]*)"`).FindStringSubmatch(tag); fm != nil {
				parts := strings.Split(fm[1], ",")
				f.paramIn = "query"
				f.paramName = parts[0]
				for _, p := range parts[1:] {
					if p == "optional" {
						f.optional = true
					}
				}
			}
			if pm := regexp.MustCompile(`path:"([^"]*)"`).FindStringSubmatch(tag); pm != nil {
				f.paramIn = "path"
				f.paramName = pm[1]
			}
			// form / path 字段属于 URL 参数，不应出现在请求体 schema 里
			if f.json == "" && f.paramIn == "" {
				f.json = f.name
			}
			if strings.Contains(tag, "optional") {
				f.optional = true
			}
			td.fields = append(td.fields, f)
		}
		return td
	}

	for _, block := range findBlock(src, "type (", '(', ')') {
		for _, st := range splitStructs(block) {
			td := parseStruct(st.name, st.body)
			types[td.name] = td
		}
	}

	// cms.api 尾部还有独立 `type Xxx { ... }` 声明，findBlock 的 "type (" 扫不到。
	// 漏掉它们会让 requestBody / 响应拼出指向未定义 schema 的悬空 $ref。
	standalone := regexp.MustCompile(`(?m)^\s*type\s+(\w+)\s*\{`)
	for _, m := range standalone.FindAllStringSubmatch(src, -1) {
		name := m[1]
		for _, body := range findBlock(src, "type "+name+" {", '{', '}') {
			td := parseStruct(name, body)
			types[name] = td
		}
	}

	// 第二遍展开匿名嵌入字段；types 收齐后才展开，避免被嵌入类型尚未解析。
	for name, td := range types {
		if len(td.embedded) == 0 {
			continue
		}
		fields := append([]field(nil), td.fields...)
		seen := make(map[string]bool, len(fields))
		for _, f := range fields {
			if f.json != "" {
				seen[f.json] = true
			}
		}
		visited := map[string]bool{name: true}
		var expand func(string)
		expand = func(emb string) {
			if visited[emb] {
				return
			}
			visited[emb] = true
			base, ok := types[emb]
			if !ok {
				return
			}
			for _, sub := range base.embedded {
				expand(sub)
			}
			for _, f := range base.fields {
				if f.json == "" || seen[f.json] {
					continue
				}
				seen[f.json] = true
				fields = append(fields, f)
			}
		}
		for _, emb := range td.embedded {
			expand(emb)
		}
		td.fields = fields
		td.embedded = nil
		types[name] = td
	}
	return types
}

func parseRoutes(src string) []route {
	var routes []route
	// 每个可选段自带前导空白：早期版本在路径后用 \s* 吃掉空格，
	// 会让「get /path returns (Resp)」这种没有请求体的路由漏掉响应类型。
	// 请求与响应都允许 []T / *T / map[string]T；早期只用 (\w+) 会让
	// `returns ([]DepartmentTreeResp)` 整段被跳过，响应模型静默退化成 object。
	re := regexp.MustCompile(`^(get|post|put|delete|patch|head|options)\s+(\S+)(?:\s+\(([^)\s]+)\))?(?:\s+returns\s*\(([^)\s]+)\))?`)
	for _, block := range findBlock(src, "service", '{', '}') {
		lines := strings.Split(block, "\n")
		var cur route
		inRoute := false
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, "@handler") {
				cur = route{handler: strings.TrimSpace(strings.TrimPrefix(line, "@handler"))}
				inRoute = true
				continue
			}
			if inRoute {
				if m := re.FindStringSubmatch(line); m != nil {
					cur.method = strings.ToUpper(m[1])
					cur.path = m[2]
					cur.req = m[3]
					cur.resp = m[4]
					routes = append(routes, cur)
					inRoute = false
				}
			}
		}
	}
	return routes
}

func toSchema(t string, types map[string]typeDef) map[string]interface{} {
	t = strings.TrimSpace(t)
	nullable := false
	if strings.HasPrefix(t, "*") {
		nullable = true
		t = t[1:]
	}
	if strings.HasPrefix(t, "[]") {
		item := toSchema(t[2:], types)
		return map[string]interface{}{"type": "array", "items": item}
	}
	if strings.HasPrefix(t, "map[string]") {
		val := toSchema(t[len("map[string]"):], types)
		return map[string]interface{}{"type": "object", "additionalProperties": val}
	}
	// 已声明的复合类型生成 $ref，避免 []PublicArticleItem 被降级为 string
	if _, ok := types[t]; ok {
		ref := map[string]interface{}{"$ref": "#/components/schemas/" + t}
		if nullable {
			return map[string]interface{}{"allOf": []interface{}{ref}, "nullable": true}
		}
		return ref
	}
	var schema map[string]interface{}
	switch t {
	case "string":
		schema = map[string]interface{}{"type": "string"}
	case "int64", "int", "int32", "int16", "uint", "uint64", "uint32":
		schema = map[string]interface{}{"type": "integer"}
	case "float64", "float32", "float":
		schema = map[string]interface{}{"type": "number"}
	case "bool":
		schema = map[string]interface{}{"type": "boolean"}
	case "time.Time":
		schema = map[string]interface{}{"type": "string", "format": "date-time"}
	default:
		// 未知但首字母大写的具名类型仍出 $ref：若解析漏了某个 type 声明，
		// 静默降级成 string 会把「悬空引用」洗白，让 $ref 完整性测试假通过。
		if t != "" && t[0] >= 'A' && t[0] <= 'Z' {
			ref := map[string]interface{}{"$ref": "#/components/schemas/" + t}
			if nullable {
				ref["nullable"] = true
			}
			return ref
		}
		schema = map[string]interface{}{"type": "string"}
	}
	if nullable {
		schema["nullable"] = true
	}
	return schema
}

func buildSchema(td typeDef, types map[string]typeDef) map[string]interface{} {
	props := map[string]interface{}{}
	required := []string{}
	for _, f := range td.fields {
		if f.json == "" {
			continue // query / path 参数不属于请求体
		}
		props[f.json] = toSchema(f.goType, types)
		if !f.optional {
			required = append(required, f.json)
		}
	}
	schema := map[string]interface{}{"type": "object", "properties": props}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func operationTag(path string) string {
	switch {
	case strings.HasPrefix(path, "/admin/articles"),
		strings.HasPrefix(path, "/admin/categories"),
		strings.HasPrefix(path, "/admin/tags"),
		strings.HasPrefix(path, "/admin/trash"):
		return "内容管理"
	case strings.HasPrefix(path, "/api/"),
		path == "/sitemap.xml", path == "/robots.txt":
		return "公开阅读"
	default:
		return "系统管理"
	}
}

func generate(src string) map[string]interface{} {
	src = stripComments(src)
	types := parseTypes(src)
	routes := parseRoutes(src)

	paths := map[string]interface{}{}
	for _, r := range routes {
		p := pathParamRe.ReplaceAllString(r.path, "{$1}")
		method := strings.ToLower(r.method)
		op := map[string]interface{}{
			"summary":     r.handler,
			"operationId": r.handler,
			"tags":        []string{operationTag(r.path)},
		}

		if r.req != "" {
			if td, ok := types[r.req]; ok {
				params := []interface{}{}
				hasBody := false
				for _, f := range td.fields {
					if f.paramIn != "" {
						params = append(params, map[string]interface{}{
							"name":     f.paramName,
							"in":       f.paramIn,
							"required": f.paramIn == "path" || !f.optional,
							"schema":   toSchema(f.goType, types),
						})
					}
					if f.json != "" {
						hasBody = true
					}
				}
				if len(params) > 0 {
					op["parameters"] = params
				}
				if hasBody {
					op["requestBody"] = map[string]interface{}{
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": toSchema(r.req, types),
							},
						},
					}
				}
			} else {
				op["requestBody"] = map[string]interface{}{
					"content": map[string]interface{}{
						"application/json": map[string]interface{}{
							"schema": toSchema(r.req, types),
						},
					},
				}
			}
		}

		// RSS / sitemap / robots 输出原始文本，不套 { code, msg, data }
		if contentType, ok := textResponseTypes[r.handler]; ok {
			op["responses"] = map[string]interface{}{
				"200": map[string]interface{}{
					"description": "成功",
					"content": map[string]interface{}{
						contentType: map[string]interface{}{
							"schema": map[string]interface{}{"type": "string"},
						},
					},
				},
			}
		} else {
			respContent := map[string]interface{}{}
			if r.resp != "" {
				respContent["schema"] = toSchema(r.resp, types)
			} else {
				respContent["schema"] = map[string]interface{}{"type": "object"}
			}
			op["responses"] = map[string]interface{}{
				"200": map[string]interface{}{
					"description": "成功",
					"content": map[string]interface{}{
						"application/json": respContent,
					},
				},
			}
		}

		pm, ok := paths[p].(map[string]interface{})
		if !ok {
			pm = map[string]interface{}{}
			paths[p] = pm
		}
		pm[method] = op
	}

	schemas := map[string]interface{}{}
	for name, td := range types {
		schemas[name] = buildSchema(td, types)
	}

	return map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":   "Meteor CMS API",
			"version": "v1",
			"description": "由 cms.api 与 content.doc.api 自动生成的 OpenAPI 文档（make gen-doc）。" +
				"除 RSS / sitemap / robots 等纯文本输出外，所有接口统一响应结构为 { code:int, msg:string, data:any }，" +
				"其中 data 即各接口标注的响应模型；query / path 参数已在 parameters 中标注。",
		},
		"servers": []interface{}{map[string]interface{}{"url": "/"}},
		"paths":   paths,
		"components": map[string]interface{}{
			"schemas": schemas,
		},
	}
}
