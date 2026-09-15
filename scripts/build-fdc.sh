#!/usr/bin/env bash
#
# FDC Store（dujiao-next）本地一键构建 —— 带自检闸门。
#
# 存在的理由：管理后台挂在自定义子路径（web.admin_path，如 /dj-mgmt-xxxx）下，
# admin 前端必须用 VITE_FULLSTACK=1 构建，否则资源被指到网站根路径 /assets/，
# 而根路径是买家站，取不到 admin 的 JS → 服务器回退返回 HTML → 浏览器白屏。
# 这个坑踩过两次，都是因为跑了裸 `npm run build`。本脚本把正确参数固化下来，
# 并在构建后校验 admin 产物，路径错了直接退出，不让坏产物流到线上。
#
# 用法（Git Bash）：  ./scripts/build-fdc.sh
# 产物：             ./build/dujiao-next（Linux amd64 单二进制）
# 部署：             见私有服务器笔记，本脚本只做本地构建+自检，不含任何凭据。
set -euo pipefail

cd "$(dirname "$0")/.."
ROOT="$(pwd)"

# C: 盘常年满，Go 缓存必须落 D:；已设过就尊重现有值。
export GOCACHE="${GOCACHE:-/d/GoCache}"
# 买家 SPA 体积大，默认堆内存会 OOM。
export NODE_OPTIONS="${NODE_OPTIONS:---max-old-space-size=4096}"

VER="${FDC_VERSION:-v1.0.0-fdc-$(date +%Y%m%d)}"
OUT="$ROOT/build/dujiao-next"
mkdir -p "$ROOT/build"

say() { printf '\n\033[1;36m==> %s\033[0m\n' "$*"; }
die() { printf '\n\033[1;31m✗ %s\033[0m\n' "$*" >&2; exit 1; }

# ---------------------------------------------------------------------------
say "1/5 构建管理后台（必须 fullstack）"
# 用 export 而不是内联 VITE_FULLSTACK=1：内联写法在 Windows cmd.exe 下会失败，
# export 在 Git Bash / Linux / mac 都稳。
( cd frontend/admin && VITE_FULLSTACK=1 npm run build )

say "自检：admin 产物路径必须是子路径安全的"
ADMIN_HTML="$ROOT/frontend/admin/dist/index.html"
[ -f "$ADMIN_HTML" ] || die "admin 没构建出 index.html"
grep -q '__DJ_ADMIN_BASE__' "$ADMIN_HTML" \
  || die "admin index.html 缺少 <base href=\"__DJ_ADMIN_BASE__/\"> 占位符——十有八九是漏了 VITE_FULLSTACK=1。已中止，不会发出白屏产物。"
grep -qE '(src|href)="\./assets/' "$ADMIN_HTML" \
  || die "admin 资源不是相对路径（./assets），fullstack 没生效。已中止。"
if grep -qE '(src|href)="/assets/' "$ADMIN_HTML"; then
  die "admin 资源指向了根路径 /assets ——这正是白屏的根因。已中止。"
fi
printf '   ✓ admin index.html 带 base 占位符、资源为 ./assets 相对路径\n'

# ---------------------------------------------------------------------------
say "2/5 构建买家端"
( cd frontend/user && npm run build )
[ -f "$ROOT/frontend/user/dist/index.html" ] || die "user 没构建出 index.html"

# ---------------------------------------------------------------------------
say "3/5 把产物搬进 internal/web/dist（go:embed 只读包目录内文件）"
rm -rf internal/web/dist/admin internal/web/dist/user
cp -r frontend/admin/dist internal/web/dist/admin
cp -r frontend/user/dist  internal/web/dist/user

# ---------------------------------------------------------------------------
say "4/5 交叉编译 Linux 单二进制"
# tags 必须带 release,fullstack。
# 注意：不要注入 version.BuildType=release —— 那会解锁一键升级，可能被上游覆盖掉本二开分支。
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -tags release,fullstack \
  -ldflags="-s -w -X github.com/dujiao-next/internal/version.Version=$VER" \
  -o "$OUT" ./cmd/server

# ---------------------------------------------------------------------------
say "5/5 二进制自检"
[ -s "$OUT" ] || die "二进制没生成"
grep -q -a -F '__DJ_ADMIN_BASE__' "$OUT" \
  || die "二进制里没有 admin base 占位符，嵌入的 admin 产物不对。已中止。"
SIZE=$(stat -c %s "$OUT" 2>/dev/null || wc -c < "$OUT")
SHA=$(sha256sum "$OUT" | cut -d' ' -f1)

printf '\n\033[1;32m✓ 构建通过\033[0m\n'
printf '   版本   %s\n' "$VER"
printf '   产物   %s\n' "$OUT"
printf '   大小   %s 字节\n' "$SIZE"
printf '   sha256 %s\n' "$SHA"
printf '\n下一步：把 build/dujiao-next 传到服务器（上传后务必两端核对 sha256），\n'
printf '        install 到 /opt/dujiao-next/bin/ 并 systemctl restart dujiao-next。\n'
printf '        部署凭据不在仓库里，见私有服务器笔记。\n\n'
