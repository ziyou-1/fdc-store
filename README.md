# FDC Store

面向东南亚市场的**虚拟商品自助发卡商城** —— AI 订阅账号、游戏点卡、软件卡密、Token 额度，下单即时发货。

后端是 Go 写的模块化单体，前端是两个独立的 Vue 3 SPA（买家端 + 管理后台）。生产部署时两个 SPA 通过 `go:embed` 打进同一个二进制，一个进程一个端口对外服务，不需要额外的 Node 运行时或前端容器。

> 本项目基于开源项目 [dujiao-next](https://github.com/dujiao-next/dujiao-next) 二次开发，遵循 GPL-3.0 协议。分叉点为上游 `a25a511`（2026-09-01）。改了什么见 [相对上游的改动](#相对上游的改动)。

---

## 目录

- [技术栈](#技术栈)
- [项目结构](#项目结构)
- [快速开始](#快速开始)
- [配置说明](#配置说明)
- [构建生产二进制](#构建生产二进制)
- [部署](#部署)
- [支付接入](#支付接入)
- [多语言](#多语言)
- [测试](#测试)
- [相对上游的改动](#相对上游的改动)
- [许可证](#许可证)

---

## 技术栈

| 层 | 选型 |
| --- | --- |
| 后端 | Go 1.26+ / Gin / GORM |
| 数据库 | SQLite（纯 Go 驱动 `glebarez/sqlite`，零 CGO）或 PostgreSQL |
| 缓存与队列 | Redis 7 + [asynq](https://github.com/hibiken/asynq) |
| 前端 | Vue 3 + TypeScript + Vite + Tailwind CSS |
| 部署形态 | 单一静态链接二进制（前端 embed），或 Docker |

规模：Go 约 1200 个文件 / 20 万行，Vue 组件 351 个，TS 模块 184 个。

## 项目结构

```
cmd/server/            程序入口
internal/
  app/                 装配层：容器、HTTP 服务、路由、后台任务
  modules/             业务模块（每个模块内部按 domain/contract/application/infrastructure/transport 分层）
  architecture/        架构守卫测试：文件预算、依赖方向、路由信任边界
  i18n/                后端多语言消息
  web/                 fullstack 构建时把前端产物 embed 进来
frontend/user/         买家商城 SPA
frontend/admin/        管理后台 SPA
```

业务模块共 37 个，主要有：

`catalog`（商品/分类/SKU）· `order`（下单/履约）· `payment`（支付网关）· `cardsecret`（卡密库存）· `fulfillment`（发货）· `wallet`（余额）· `points`（积分）· `memberlevel`（会员）· `coupon` · `promotion` · `giftcard` · `affiliate`（推广返利）· `reseller`（分销白标）· `support`（客服工单）· `freeclaim`（免费领取）· `tokenstation`（Token 额度）· `procurement`（采购）· `reconciliation`（对账）· `dashboard` · `reporting` · `notification` · `telegram` · `settings` · `identity` · `authz`

每个模块都遵循同一套分层，跨模块只能通过 `contract` 包定义的端口通信，`internal/architecture/` 下的守卫测试会在 CI 里挡住越界依赖。

## 快速开始

前置：Go 1.26+、Node 20+、pnpm、Redis 7。

```bash
# 1. 配置
cp config.yml.example config.yml
# 编辑 config.yml，至少要把下面三个密钥改成三个互不相同的强随机值，否则服务拒绝启动：
#   app.secret_key / jwt.secret / user_jwt.secret
# 生成方式（跑三次）：
openssl rand -hex 32

# 2. 起 Redis（必需，见下方说明）
redis-server

# 3. 后端
go run ./cmd/server

# 4. 买家端（另开终端）
cd frontend/user && pnpm install && pnpm dev

# 5. 管理后台（再开一个终端）
cd frontend/admin && pnpm install && pnpm dev
```

首次启动会自动跑数据库迁移并建表。管理员账号由 `bootstrap.default_admin_username` / `default_admin_password` 在初始化时创建——**这两项留空的话一个管理员都不会建，而且没有注册入口，等于进不去后台**，首次启动前务必填好。

健康检查：`GET /health` 返回 `{"status":"ok"}`。

### Redis 是硬依赖

`queue.enabled=true` 且默认 `-mode=all` 时，Redis 连不上会直接导致进程退出。唯一的无 Redis 方案是 `queue.enabled=false` **且** `-mode=api`，但代价是下列周期任务**永远不会注册**：

- 订单支付超时自动取消（发卡站的核心逻辑）
- 会员到期清理与到期提醒
- 分销佣金确认入账
- 上游库存同步与库存告警

另外关掉 Redis 会让基于 `SetNX` 的幂等锁全部失效。**生产环境请老老实实装 Redis。**

## 配置说明

配置从**进程工作目录**按 `config.yml` 查找（不是二进制所在目录）。SQLite 库文件、`uploads/`、`logs/` 也都是相对工作目录解析的，所以 systemd 单元必须设置 `WorkingDirectory`，否则会在别的目录凭空生成一套空数据，表现为「数据全没了」。

关键项：

| 配置 | 说明 |
| --- | --- |
| `app.secret_key` | AES-256 主密钥，加密库内支付/上游凭证。**换了它就解不开老数据** |
| `jwt.secret` / `user_jwt.secret` | 后台与买家的 Token 签名密钥 |
| `server.host` | 有反向代理时填 `127.0.0.1`，避免绕过 Nginx 直连 |
| `server.trusted_proxies` | 只填真实代理地址，禁止 `0.0.0.0/0` |
| `database.driver` | `sqlite` 或 `postgres`（**不支持 mysql**，填了会启动失败） |
| `database.pool.max_open_conns` | SQLite 下刻意设为 1，不要「优化」成更大值 |
| `web.admin_path` | 后台路径前缀，强烈建议改成不易猜测的字符串 |
| `cors.allowed_origins` | 默认 `*` 且 `allow_credentials: true`，生产必须收敛成实际来源 |

三个密钥必须各自 ≥32 字符、互不相同、且不含 `change-me` 之类的占位词，否则启动被拦。

`release` 模式比 `debug` 更严格：弱管理员口令在 debug 下只告警，在 release 下直接拒绝启动。

## 构建生产二进制

生产形态是**一个静态链接的二进制**，两个 SPA 已经 embed 在里面。构建链有三个不显然的点，漏一个就得重来：

```bash
# 1) admin 必须走 fullstack 变体
#    它把 base 改成相对路径并注入 <base href="__DJ_ADMIN_BASE__/"> 占位符，
#    后端启动时按 web.admin_path 替换，同一份产物才能挂到任意自定义前缀。
cd frontend/admin && pnpm install && VITE_FULLSTACK=1 pnpm run build && cd ../..

# 2) user 走普通变体
cd frontend/user && pnpm install && pnpm run build && cd ../..

# 3) 产物必须拷到 internal/web/dist/ —— go:embed 只能读包目录内的文件
rm -rf internal/web/dist && mkdir -p internal/web/dist
cp -r frontend/admin/dist internal/web/dist/admin
cp -r frontend/user/dist  internal/web/dist/user

# 4) 编译（CGO_ENABLED=0 可行，因为 SQLite 用的是纯 Go 驱动）
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -tags release,fullstack \
  -ldflags="-s -w -X github.com/dujiao-next/internal/version.Version=v1.0.0" \
  -o out/dujiao-next ./cmd/server
```

验收：

```bash
file out/dujiao-next                        # 应为 statically linked ELF
grep -a -c __DJ_ADMIN_BASE__ out/dujiao-next # >=1 说明 admin SPA 真的进去了
```

几个坑：

- **Windows 下不要直接 `pnpm run build:fullstack`**：脚本里的内联 `VITE_FULLSTACK=1` 由 cmd.exe 执行会失败，改成在父进程设好环境变量再调普通 `build`。
- **不要注入 `-X .../version.BuildType=release`**：那会让 selfupdate 把这个二进制判定为「官方正式版」，后台「一键升级」随之解锁，而升级源写死指向上游仓库——点一下就把二开成果覆盖成上游官方版。保持默认的 `source` 即可永久阻断。
- `-tags release` 要保留（它把广告网关等指向生产地址），与上面的 `BuildType` 是两回事。

## 部署

推荐：本机交叉编译 → 传一个二进制 → systemd 常驻 → Nginx 反代。

```ini
[Unit]
Description=FDC Store
After=network-online.target redis.service
Wants=network-online.target redis.service

[Service]
Type=simple
User=dujiao
WorkingDirectory=/opt/dujiao-next
ExecStart=/opt/dujiao-next/bin/dujiao-next
Restart=always
RestartSec=5s

NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/dujiao-next

[Install]
WantedBy=multi-user.target
```

`Wants=redis.service` 而不是 `Requires=`：用 `Requires` 时 Redis 一停，systemd 会连带停掉本服务，且 Redis 恢复后**不会**自动把它拉回来（依赖性停止不触发 `Restart=always`），一次 Redis 升级就能让商城永久下线。用 `Wants` 让 go-redis / asynq 自己重连。

Nginx 侧要点：透传真实 `Host`（分销白标按 Host 解析站点上下文）、带上 WebSocket 升级头（客服 IM 要用）、`client_max_body_size` 放开一点。

需要备份的是三样，且**必须同批备份同批恢复**：

- `db/` —— SQLite 开了 WAL，运行中直接 `cp` 可能拿到不一致快照，热备请用 `sqlite3 db/dujiao.db ".backup out.db"`
- `uploads/` —— 商品图与客服附件
- `config.yml` —— 里面的 `app.secret_key` 是库内敏感字段的解密密钥，拿新的配置文件配老库会解不开

## 支付接入

已实现的网关适配器：

`payssion`（东南亚本地钱包）· `stripe` · `paypal` · `alipay` · `wechatpay` · `epay` · `epusdt` · `bepusdt` · `okpay` · `tokenpay` · `dujiaopay`

另有三种不走网关的内置支付方式：钱包余额、积分抵扣、免费领取。

支付与财务相关的后台路由挂在合规声明中间件之后，超管需要先在后台确认一次合规声明才能访问。

**回调地址不在配置文件里**：每个渠道的 `notify_url` / `return_url` 是在后台逐个渠道配置的，站点 URL 是数据库设置 `brand.site_url`。换域名时改 `config.yml` 没用，必须去后台逐个改。

## 多语言

买家端支持 简体中文 / 繁体中文 / English / Bahasa Indonesia 四种语言，默认语言静态打包，其余按需懒加载。后端错误消息按 `X-Lang` 请求头返回对应语言。

新增一门语言要改的地方比看上去多，而且**漏改基本不会编译报错，只会静默降级**（后台没有输入框、内容渲染成空、多语言搜索少一列）。至少包括：

- `internal/constants` 的 `SupportedLocales`（改这里会自动带动约 12 处调用点）
- `internal/i18n/messages.go` 的消息块与 `normalizeLocale` 的语言协商分支
- `internal/modules/settings/schema/messaging/notification.go` 里另有一张独立白名单
- 买家端 `i18n/index.ts` 的三处（支持列表、懒加载器、浏览器语言探测）与两个语言切换器
- `utils/sku.ts` 的 `SUPPORTED_LOCALES` —— 这里最容易踩：`isLocalizedObject` 要求对象的*每个* key 都在表里，漏了会让含新语言的商品标题被判定为「不是多语言对象」，SKU 名直接渲染错
- 后台所有多语言表单的空值初始化与内容录入 tab

## 测试

```bash
go test ./...                                   # 后端
cd frontend/user  && pnpm test                  # 买家端
cd frontend/admin && npx vitest run             # 后台
```

`internal/architecture/` 下是架构守卫测试，会检查模块结构、依赖方向、路由信任边界、文件数预算——重构时它们先红是正常的，说明动到了约定。

已知的环境相关失败（非代码问题）：`CGO_ENABLED=0` 时若某些测试走了 `mattn/go-sqlite3` 会跳过失败；Windows 下 logger 与 selfupdate 有文件锁与 POSIX 权限相关的失败。

## 相对上游的改动

分叉自上游 `a25a511`（2026-09-01）。主要改动：

**新增业务模块**

- `points` —— 积分体系：签到、推荐奖励、充值赠分、下单抵扣
- `freeclaim` —— 限量免费领取
- `support` —— 站内客服工单与实时会话
- `tokenstation` —— Token 额度商品

**支付**

- 新增 Payssion 网关适配器，接入东南亚本地钱包（QRIS / OVO / Dana / ShopeePay / Touch'n Go / GCash / PicPay）
- 支持「结算货币入账」模式：这类回调里 `amount`/`currency` 是换算后的入账值，原值在 `amount_local`/`currency_local`，对账必须回到原值，否则每一笔合法支付都会被判币种不符
- 补齐官方状态表里的终态失败 `rejected` / `blocked`，避免这两类交易永远停在 pending
- 支付渠道的网关配置校验改为「只对启用中的渠道生效」——原先无条件校验，导致配置不全的渠道**永远关不掉**
- 种子数据不再复活已被管理员删除的支付渠道（原先每次重启都会以「启用」状态重建）

**前端与国际化**

- 新增印尼语（id-ID），买家端与后端消息共约 1900 条
- 会员续费改为时长叠加而非覆盖（原先「还剩 300 天时续一个月」会把 300 天抹成 30 天）
- 移除上游的后台广告位与广告代理模块（后台原本每次打开都会请求上游广告网关并回传曝光数据）
- 下线博客与免登录下单/查单

## 许可证

本项目继承上游的 [GPL-3.0](LICENSE) 协议。

这意味着：你可以自由使用、修改和分发本项目，但**分发修改版时必须同样以 GPL-3.0 开源，并保留版权声明、标注你所做的修改**。

上游项目：[dujiao-next/dujiao-next](https://github.com/dujiao-next/dujiao-next)
