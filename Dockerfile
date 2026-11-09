# ---------- 前端构建阶段 ----------
# NPM_REGISTRY 可覆盖为镜像源（如 https://registry.npmmirror.com/）以适配受限网络环境
FROM node:22-alpine AS web
ARG NPM_REGISTRY=https://registry.npmjs.org/
WORKDIR /web
# 连同 lock 一起复制并使用 npm ci，保证镜像构建与 CI / 本地使用同一份依赖树（发布可复现）
COPY web/package.json web/package-lock.json ./
# replace-registry-host=always：lock 中 resolved 固化了生成时的源（当前为 npmmirror），
# 该参数让 --registry / NPM_REGISTRY 真正生效，避免不同环境被迫走同一个源。
RUN npm ci --replace-registry-host=always --registry=$NPM_REGISTRY --no-audit --no-fund
COPY web/ ./
# 产物位于 /web/dist（vite 默认 base 为 /，由 Go 服务统一托管 SPA）
RUN npm run build

# ---------- 后端构建阶段 ----------
# GOPROXY 可覆盖为镜像源（如 https://goproxy.cn,direct）以适配受限网络环境
FROM golang:1.26-alpine AS builder

ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=$GOPROXY

# 构建版本信息：由发布流水线注入，本地 docker build 使用 dev 默认值。
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
ARG LDFLAGS_PKG=github.com/yuwen002/meteor-cms/internal/version

WORKDIR /src

# 先复制依赖清单，利用镜像层缓存加速后续构建
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Go 直接从目录托管前端静态文件，无需 embed；dist 由运行阶段挂载
RUN LDFLAGS="-s -w -X ${LDFLAGS_PKG}.Version=${VERSION} -X ${LDFLAGS_PKG}.Commit=${COMMIT} -X ${LDFLAGS_PKG}.BuildTime=${BUILD_TIME}" \
 && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "$LDFLAGS" -o /out/cms ./api/cms/v1 \
 && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "$LDFLAGS" -o /out/migrate ./cmd/migrate \
 && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "$LDFLAGS" -o /out/backup ./cmd/backup

# ---------- 运行阶段 ----------
FROM alpine:3.20

RUN apk add --no-cache tzdata ca-certificates \
 && adduser -D -u 10001 app

WORKDIR /app

COPY --from=builder /out/cms /app/cms
COPY --from=builder /out/migrate /app/migrate
COPY --from=builder /out/backup /app/backup
COPY --from=web /web/dist /app/web/dist
# 将 OpenAPI 文档一并发布到静态目录，供 /docs.html（Swagger UI）读取
COPY openapi.json /app/web/dist/openapi.json
COPY api/cms/v1/etc/cms-api.yaml /app/etc/cms-api.yaml

RUN chown -R app:app /app
USER app

ENV TZ=Asia/Shanghai
# 由 Go 服务直接托管的前端产物目录（SPA + history 路由回退）
ENV WEB_DIST=/app/web/dist

EXPOSE 8888

CMD ["/app/cms", "-f", "etc/cms-api.yaml"]
