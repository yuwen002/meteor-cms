.PHONY: help build run test test-integration bench cover fmt vet lint tidy migrate migrate-check migrate-dry-run migrate-schema migrate-seed migrate-backfill version gen-api gen-ent gen-doc backup restore docker-build docker-up docker-down clean

BINARY_DIR := bin
API_PKG    := ./api/cms/v1
BENCHTIME  ?= 200x

help: ## 显示可用命令
	@echo "可用命令:"
	@echo "  build         编译 API 服务、迁移工具与备份工具到 bin/"
	@echo "  run           启动 API 服务"
	@echo "  test          运行全部测试"
	@echo "  test-integration  旧版 schema 升级集成测试（需 MYSQL_DSN，使用独立库）"
	@echo "  bench         运行热点路径基准（可用 BENCHTIME=2s 覆盖，默认 200x）"
	@echo "  cover         生成覆盖率报告"
	@echo "  fmt           格式化代码"
	@echo "  vet           静态检查"
	@echo "  lint          格式化 + 静态检查"
	@echo "  tidy          整理依赖"
	@echo "  migrate       执行完整迁移（结构 + 种子 + 版本记录，幂等）"
	@echo "  migrate-check 只读检查表结构漂移，有差异时退出码 2"
	@echo "  migrate-dry-run  输出待新增表/列与期望索引清单，不写库"
	@echo "  migrate-schema   只同步表结构，跳过种子"
	@echo "  migrate-seed     只重跑种子（表结构已就绪）"
	@echo "  migrate-backfill 回填历史上传文件的媒体元数据（UPLOAD_DIR 可覆盖目录）"
	@echo "  version      打印构建版本与期望 schema 版本"
	@echo "  backup        导出内容快照 JSON（可指定 FILE=路径）"
	@echo "  restore       从内容快照恢复（需 FILE=路径，OVERWRITE=true 覆盖同 slug 文章）"
	@echo "  gen-api       按 cms.api 重新生成 API 代码"
	@echo "  gen-ent       重新生成 Ent 代码"
	@echo "  gen-doc       生成全量 OpenAPI 文档（输出 openapi.json）"
	@echo "  docker-build  构建镜像"
	@echo "  docker-up     启动 MySQL + 迁移 + API"
	@echo "  docker-down   停止容器"
	@echo "  clean         清理构建产物（不会删除 backup/）"

build: ## 编译 API 服务、迁移工具与备份工具到 bin/
	@mkdir -p $(BINARY_DIR)
	go build -o $(BINARY_DIR)/cms $(API_PKG)
	go build -o $(BINARY_DIR)/migrate ./cmd/migrate
	go build -o $(BINARY_DIR)/backup ./cmd/backup

run: ## 启动 API 服务
	go run $(API_PKG)/cms.go

test: ## 运行全部测试
	go test ./...

test-integration: ## 旧版 schema 升级集成测试（必须设置 MYSQL_DSN）
	go test -tags=integration -count=1 -v ./test/migration/...

bench: ## 运行全部基准（跳过单测，输出耗时与分配；BENCHTIME 可覆盖）
	go test ./... -run=^$$ -bench=. -benchmem -benchtime=$(BENCHTIME)

cover: ## 生成覆盖率报告并打开
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

fmt: ## 格式化代码
	gofmt -w api cmd internal ent/schema

vet: ## 静态检查
	go vet ./...

lint: fmt vet ## 格式化 + 静态检查

tidy: ## 整理依赖
	go mod tidy

migrate: ## 执行完整迁移（结构 + 种子 + 版本记录，幂等）
	go run ./cmd/migrate

migrate-check: ## 只读检查表结构漂移（有差异时退出码 2）
	go run ./cmd/migrate -check

migrate-dry-run: ## 输出待新增表/列与期望索引清单，不执行 DDL
	go run ./cmd/migrate -dry-run

migrate-schema: ## 只同步表结构，跳过种子
	go run ./cmd/migrate -skip-seed

migrate-seed: ## 只重跑种子（表结构已就绪）
	go run ./cmd/migrate -only-seed

migrate-backfill: ## 回填历史上传文件的媒体元数据（UPLOAD_DIR 可覆盖目录）
	go run ./cmd/migrate -only-seed -backfill $(if $(UPLOAD_DIR),-upload-dir $(UPLOAD_DIR),)

version: ## 打印构建版本与期望 schema 版本
	go run ./cmd/migrate -version

backup: ## 导出内容快照到 backup/（FILE=... 可指定路径）
	go run ./cmd/backup -mode export $(if $(FILE),-file $(FILE),)

restore: ## 从内容快照恢复（FILE=... 必填，OVERWRITE=true 覆盖同 slug 文章）
	go run ./cmd/backup -mode import -file $(FILE) $(if $(filter true,$(OVERWRITE)),-overwrite,)

gen-api: ## 按 cms.api 重新生成 API 代码（需 goctl 1.10.2，必须指定 --home .goctl）
	goctl api go -api $(API_PKG)/cms.api -dir $(API_PKG) --home .goctl

gen-ent: ## 重新生成 Ent 代码
	go run -mod=mod entgo.io/ent/cmd/ent generate --feature intercept,schema/snapshot ./ent/schema

gen-doc: ## 由 cms.api + 手工路由文档源生成全量 OpenAPI 文档（输出 openapi.json）
	go run ./cmd/apidoc

docker-build: ## 构建镜像
	docker build -t meteor-cms:local .

docker-up: ## 启动全栈：MySQL + Redis + 迁移 + API（依赖顺序由 depends_on 保证）
	docker compose up -d

docker-down: ## 停止并清理容器
	docker compose down

clean: ## 清理构建产物（保留 backup/ 下的备份）
	rm -rf $(BINARY_DIR) coverage.out
