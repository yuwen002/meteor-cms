#!/usr/bin/env bash
# 生产部署前检查：与 api 的 STRICT_SECURITY 启动校验、GHCR 拉取、迁移备份链路对齐。
#
# 用法（推荐 make 入口；脚本以 bash 调用，无需可执行位）：
#   make preflight
#   ENV_FILE=/etc/meteor-cms/.env make preflight
#   SKIP_IMAGE_CHECK=1 make preflight                       # 离线 / 镜像未发布时跳过可达性
#   SKIP_BACKUP_CHECK=1 make preflight                      # 首次部署尚无库可备份时跳过
#   READY_URL=http://127.0.0.1:8888/readyz make preflight   # 部署后追加就绪检查
#   bash scripts/preflight.sh                              # 不经 make 直接调用
#
# 退出码：0 = 无 FAIL（可能有 WARN）；1 = 存在 FAIL，禁止部署。
set -uo pipefail

ENV_FILE="${ENV_FILE:-.env}"
fails=0
warns=0

pass() { printf '[PASS] %s\n' "$1"; }
fail() { printf '[FAIL] %s\n' "$1"; fails=$((fails + 1)); }
warn() { printf '[WARN] %s\n' "$1"; warns=$((warns + 1)); }

# 与 docker compose 的插值优先级一致：进程环境变量 > .env。
# 不 source .env，避免执行其中任意 shell 代码。
get() {
  local key="$1" val=""
  val="$(printenv "$key" 2>/dev/null || true)"
  if [ -n "$val" ]; then
    printf '%s' "$val"
    return
  fi
  [ -f "$ENV_FILE" ] || return 0
  sed -n "s/^[[:space:]]*${key}[[:space:]]*=[[:space:]]*//p" "$ENV_FILE" \
    | tail -n 1 \
    | sed -e 's/[[:space:]]*$//' -e 's/^"//' -e 's/"$//'
}

is_placeholder() {
  case "$1" in
    ''|*CHANGE_ME*|*REPLACE_WITH*|*YOUR_*|*'<'*|*'>'*) return 0 ;;
    *) return 1 ;;
  esac
}

# 与 api/cms/v1/internal/config/config.go 的示例密钥保持一致。
EXAMPLE_ACCESS='8x!A%D*G-KaPdSgVkYp3s6v9y$B?E(H+MbQeThWmZq4t7w!z%C*F-J@NcRfUjXn2'
EXAMPLE_REFRESH='rEfReSh-6KpVq2Jx!YmN3sLz8Wb@TcE5HgU7DfA1'

printf '部署前检查  ENV_FILE=%s\n\n' "$ENV_FILE"
if [ -f "$ENV_FILE" ]; then
  pass "配置文件存在：$ENV_FILE"
else
  fail "找不到配置文件：$ENV_FILE（可用 ENV_FILE 指定路径）"
fi

access="$(get AUTH_ACCESS_SECRET)"
refresh="$(get AUTH_REFRESH_SECRET)"

if is_placeholder "$access"; then
  fail "AUTH_ACCESS_SECRET 未填写（仍是模板占位符）"
elif [ "${#access}" -lt 16 ]; then
  fail "AUTH_ACCESS_SECRET 长度不足 16 位"
elif [ "$access" = "$EXAMPLE_ACCESS" ]; then
  fail "AUTH_ACCESS_SECRET 仍是仓库示例值"
else
  pass "AUTH_ACCESS_SECRET 已自定义且长度合规"
fi

if is_placeholder "$refresh"; then
  fail "AUTH_REFRESH_SECRET 未填写（仍是模板占位符）"
elif [ "${#refresh}" -lt 16 ]; then
  fail "AUTH_REFRESH_SECRET 长度不足 16 位"
elif [ "$refresh" = "$EXAMPLE_REFRESH" ]; then
  fail "AUTH_REFRESH_SECRET 仍是仓库示例值"
elif [ "$refresh" = "$access" ]; then
  fail "两个 JWT 密钥不能相同"
else
  pass "AUTH_REFRESH_SECRET 已自定义且与 AccessSecret 不同"
fi

if [ "$(get STRICT_SECURITY)" = "true" ]; then
  pass "STRICT_SECURITY=true"
else
  fail "STRICT_SECURITY 必须为 true（否则安全弱配置只会告警而不会拦截）"
fi

# 以下四项与 SecurityWarnings() 一致：任一告警在严格模式下都会让 api 拒绝启动。
for key in LOGIN_CAPTCHA_ENABLED LOGIN_RATE_LIMIT_ENABLED PUBLIC_RATE_LIMIT_ENABLED; do
  value="$(get "$key")"
  case "${value:-true}" in
    false|FALSE|False|0|no|NO) fail "$key 被关闭（当前 $value），严格模式下会拒绝启动" ;;
    *) pass "$key 保持开启（${value:-默认 true}）" ;;
  esac
done

comment_max="$(get PUBLIC_COMMENT_RATE_MAX)"
case "${comment_max:-5}" in
  ''|0|-[0-9]*) fail "PUBLIC_COMMENT_RATE_MAX 必须 > 0（当前 ${comment_max}），严格模式下会拒绝启动" ;;
  *) pass "公开评论限流已开启（${comment_max:-5} 次/窗口）" ;;
esac

# 与 SecurityWarnings() 对齐：开启 /metrics 但未设 token 时严格模式会拒绝启动。
if [ "$(get METRICS_ENABLED)" != "true" ]; then
  pass "Prometheus /metrics 保持关闭（默认）"
else
  metrics_token="$(get METRICS_TOKEN)"
  if is_placeholder "$metrics_token" || [ -z "$metrics_token" ]; then
    fail "METRICS_ENABLED=true 时必须设置 METRICS_TOKEN（严格模式会拒绝启动）"
  elif [ "${#metrics_token}" -lt 16 ]; then
    fail "METRICS_TOKEN 长度不足 16 位"
  else
    pass "Prometheus /metrics 已开启且 token 合规"
  fi
fi

case "$(get SQL_DEBUG)" in
  true|TRUE|True|1) fail "SQL_DEBUG=true 会记录 SQL 参数，严格模式下会拒绝启动" ;;
  *) pass "SQL_DEBUG 未开启" ;;
esac

dsn="$(get MYSQL_DSN)"
if is_placeholder "$dsn"; then
  fail "MYSQL_DSN 仍是模板占位符（改口令后需同步更新）"
elif [ -z "$dsn" ]; then
  warn "MYSQL_DSN 未设置，将沿用 compose 默认值"
else
  pass "MYSQL_DSN 已配置"
fi
case "$dsn" in
  *'root:123456@'*) warn "MYSQL_DSN 仍使用仓库示例弱口令，生产应更换" ;;
esac

root_pw="$(get MYSQL_ROOT_PASSWORD)"
if is_placeholder "$root_pw"; then
  fail "MYSQL_ROOT_PASSWORD 仍是模板占位符"
elif [ "$root_pw" = "123456" ]; then
  warn "MYSQL_ROOT_PASSWORD 仍是示例弱口令，生产应更换"
elif [ -n "$root_pw" ]; then
  pass "数据库 root 口令已自定义"
fi

image="$(get CMS_IMAGE)"
case "$image" in
  '')
    fail "CMS_IMAGE 未设置，compose 会回退到本机构建的 meteor-cms:local"
    ;;
  ghcr.io/yuwen002/meteor-cms:*)
    tag="${image##*:}"
    if [ "$tag" = "latest" ]; then
      warn "CMS_IMAGE 使用 latest，回滚时无法定位版本，建议固定 X.Y.Z"
    elif printf '%s' "$tag" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$'; then
      pass "CMS_IMAGE 指向新仓库固定版本：$image"
    else
      fail "镜像 tag 应为 X.Y.Z（如 1.2.3）或 latest：$tag"
    fi
    ;;
  *)
    fail "CMS_IMAGE 未指向 ghcr.io/yuwen002/meteor-cms:<版本>：$image"
    ;;
esac

backup="$(get CMS_BACKUP_FILE)"
if [ "${SKIP_BACKUP_CHECK:-0}" = "1" ]; then
  warn "已按 SKIP_BACKUP_CHECK=1 跳过备份新鲜度检查（仅适用于首次部署 / 空库）"
elif is_placeholder "$backup" || [ -z "$backup" ]; then
  fail "未指定有效 CMS_BACKUP_FILE，请先 make backup 并在 .env 填写路径"
elif [ ! -s "$backup" ]; then
  fail "备份文件不存在或为空：$backup"
elif [ -z "$(find "$backup" -mmin -1440 -print 2>/dev/null)" ]; then
  fail "备份已超过 24 小时，请重新 make backup：$backup"
else
  pass "备份新鲜（24 小时内）：$backup"
fi

if [ "${SKIP_IMAGE_CHECK:-0}" = "1" ]; then
  warn "已按 SKIP_IMAGE_CHECK=1 跳过 GHCR 镜像可达性检查"
elif ! command -v docker >/dev/null 2>&1; then
  warn "未找到 docker，跳过 GHCR 镜像可达性检查"
elif ! printf '%s' "$image" | grep -q '^ghcr.io/yuwen002/meteor-cms:'; then
  warn "CMS_IMAGE 无效，跳过 GHCR 镜像可达性检查"
elif docker manifest inspect "$image" >/dev/null 2>&1; then
  pass "GHCR 镜像可访问：$image"
else
  fail "无法访问 $image。常见原因：(1) 生产机未登录 GHCR，或 PAT 缺少 read:packages / 已过期（见 README 的 docker login 命令）；(2) 镜像尚未发布或 tag 不存在；(3) Docker 守护进程未运行。用 docker manifest inspect $image 复查；发版前 / 离线可设 SKIP_IMAGE_CHECK=1"
fi

if [ -n "${READY_URL:-}" ]; then
  if command -v curl >/dev/null 2>&1 &&
    curl -fsS --max-time 5 "$READY_URL" 2>/dev/null | grep -q '"ready"[[:space:]]*:[[:space:]]*true'; then
    pass "服务就绪：$READY_URL"
  else
    fail "服务未就绪：$READY_URL"
  fi
fi

printf '\n剩余人工步骤：\n'
printf '  1. make migrate-check                          # 确认表结构无漂移（差异时退出码 2）\n'
printf '  2. make migrate-safe                           # 带 -require-backup 的完整迁移\n'
printf '  3. docker compose pull && docker compose up -d  # 不要加 --build\n'
printf '  4. 首次登录立即修改 admin/123456\n\n'

if [ "$fails" -gt 0 ]; then
  printf '结果：%d 项失败 / %d 项警告 —— 禁止部署。\n' "$fails" "$warns"
  exit 1
fi
printf '结果：全部通过（%d 项警告）—— 可以部署。\n' "$warns"
