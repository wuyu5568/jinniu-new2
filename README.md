# 金牛协议（jinniu）

Go + Kratos v2 后端。数据库以 `scripts/schema.sql` 为准，**禁止** GORM AutoMigrate。

## 环境要求

- Go 1.25+
- MySQL 8.x（本地或 Docker）

## 启动 MySQL

```bash
docker run -d --name jinniu-mysql -p 3306:3306 -e MYSQL_ROOT_PASSWORD=root mysql:8
mysql -h127.0.0.1 -uroot -proot -e "CREATE DATABASE IF NOT EXISTS jinniu DEFAULT CHARSET utf8mb4;"
mysql -h127.0.0.1 -uroot -proot jinniu < scripts/schema.sql
```

`schema.sql` 创建 `users`、`locations`（认购订单）、`withdraws`（提取资产）、`user_recommends`、`ledger_entries`、`business_configs` 等表，并写入默认业务参数。

## 配置

编辑 `app/app/configs/config.yaml`：

| 项 | 说明 |
|----|------|
| `data.database.source` | MySQL DSN |
| `auth.jwt_key` | 用户/管理 JWT 签名密钥 |
| `auth.admin_username` | 管理后台登录用户名 |
| `auth.admin_password` | 管理后台登录密码（MVP 明文） |
| `auth.challenge_ttl` | 可选：v1 nonce 登录有效期（如 `300s`） |
| `app.genesis_address` | 创世钱包地址（小写），首次注册可不填邀请码 |
| `app.settle_cron` | 日结 cron 表达式（空则关闭定时） |
| `app.settle_timezone` | 日结时区（默认 Asia/Shanghai） |
| `app.allow_force_settle` | 是否允许 `force=1` 强跑日结（生产务必 `false`；本地/e2e 可 `true`） |
| `app.payout_enabled` | 是否允许提取链上打款（默认 `false`；也可用环境变量 `JINNIU_PAYOUT_ENABLED=1` 覆盖）。**开启时必须**同时设 `JINNIU_PAYOUT_MAX_USDT>0`，否则拒绝启动 |
| `app.payout_cron` | 打款队列 cron（空则仅管理端触发） |
| `app.deposit_cron` | 链上充值自动拉取 cron（空则仅管理端触发；默认 `*/1 * * * *`） |
| `app.bsc_rpc` / `usdt_address` / `hot_wallet_key` | BSC RPC、USDT 合约、热钱包私钥（勿提交仓库；私钥优先用 `JINNIU_HOT_WALLET_KEY`） |

## 运行

```bash
go mod tidy
go build -o bin/app.exe ./app/app/cmd/app
./bin/app.exe -conf app/app/configs/config.yaml
```

HTTP 默认 `0.0.0.0:8000`，健康检查 `GET /health`（含 MySQL ping：`db=ok`；库不通返回 503）。

### 用户 API（taurus 兼容）

前缀 `/api/app_server`（JWT Bearer，除 `eth_authorize`）：

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/eth_authorize` | 钱包签名登录（签名为地址本身，字段 `address`/`code`/`sign`） |
| GET | `/user_info` | 用户信息（taurus 字段映射） |
| POST | `/buy` | 余额认购 |
| GET | `/order_list` | 认购订单列表 |
| POST | `/withdraw` | 提取资产 |
| GET | `/withdraw_list` | 提取记录 |
| POST | `/withdraw_cancel` | 取消待审提取（退回可提） |
| GET | `/recommend_list` | 直推列表 |
| GET | `/reward_list` | 收益流水 |
| GET | `/deposit_list` | 充值记录 |

另保留 `/api/app_server/v1/...` REST 路由（nonce 登录等）。

### 管理 API（dapp-admin 兼容）

前缀 `/api/admin_jinniu`（管理 JWT Bearer）：

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/login` | 用户名密码登录 → `{ token }` |
| GET | `/my_auth_list` | 菜单权限 |
| GET | `/all` | 仪表盘 KPI |
| GET | `/user_list` | 用户列表 |
| GET | `/buy_list` | 认购列表 |
| GET | `/withdraw_list` | 提取列表 |
| POST | `/withdraw_pass` | 审核通过提取 |
| GET | `/settle_status` | 今日/最近一次日结快照 |
| GET | `/payout_status` | 打款开关/热钱包地址/队列计数（不查链上余额） |
| GET/POST | `/config`, `/config_update` | 业务配置 |
| GET | `/reward_list` | 分红流水 |
| GET | `/record_list` | 充值列表 |
| POST | `/deposit`, `/add_money_three` | 账户余额：POST 人工增加；`add_money_three` 设目标值 |
| GET | `/deposit` | 拉链上充值合约入账（对齐 new18new，需管理 JWT） |
| POST | `/deposit_replay` | 按合约序号重放 skipped→入账（补注册后补账） |
| POST | `/add_money_two` | 可提余额设目标值 |

另保留 `/api/admin_jinniu/v1/...` REST 路由（同样使用管理 JWT）。

默认管理账号见 `config.yaml`（`admin` / `admin123`）。

## 链上充值（对齐 new18new）

1. 用户端 `/recharge`：授权 USDT 后调用合约 `buy(金额)`（合约 `0x0f299470…` 分账版；最低 5 USDT）
2. 管理端「账户余额充值」页点 **拉链上充值**，或 `GET /api/admin_jinniu/deposit`（管理 JWT）
3. 回执含 `pulled/credited/skipped/errors/caught_up` 与游标前后；`?until_caught_up=1` 可多轮追上（上限 5min/500）；迁移：`scripts/migrate_eth_deposit_cursor.sql`
4. 详见 [docs/adr/0007-chain-deposit-new18new.md](docs/adr/0007-chain-deposit-new18new.md)

人工 `POST /deposit` 仍可直接增加账户余额。

详见 [docs/adr/0001-frontend-compat.md](docs/adr/0001-frontend-compat.md)。  
提取链上打款（ADR 0010，默认关）：[docs/adr/0010-withdraw-onchain-payout.md](docs/adr/0010-withdraw-onchain-payout.md)。  
完整上线（含打款）：[docs/ops-go-live.md](docs/ops-go-live.md)；打款日常：[docs/ops-payout.md](docs/ops-payout.md)。迁移：`scripts/migrate_withdraw_payout.sql`。

可观测：`GET /api/admin_jinniu/payout_status`（队列与配置快照，不查链上余额；字段亦挂在 `GET /all`）。

本地联调打款（环境变量，勿提交私钥）：

```powershell
$env:JINNIU_PAYOUT_ENABLED = "1"
$env:JINNIU_HOT_WALLET_KEY = "<你的热钱包私钥hex>"
$env:JINNIU_PAYOUT_MAX_USDT = "1"   # B1 安全顶：到账额 >1 拒绝打款
# 可选: $env:JINNIU_BSC_RPC = "https://bsc-dataseed.binance.org/"
.\bin\app.exe -conf app/app/configs/config.yaml
```

**B1 小额真打注意**：默认最低提取 10，到账约 9.4，会被 `PAYOUT_MAX=1` 拦住。请临时把业务参数 `min_withdraw_amount` 改为 `1`，用户提取 **1 USDT**（到账约 0.94），再审核通过 → 打款。热钱包需有少量 USDT + BNB gas。

管理端：通过 → `rewarded` →「打款」。超额返回 `BIZ_PAYOUT_ABOVE_MAX`；余额/gas 不足会报链上错误。测完清除环境变量并重启；把最低提取额改回。

## 第 4 期收口（前端对接）

后端兼容层已覆盖 **dapp-admin / taurus 主路径**（登录、看板、用户/认购/提取、结算、配置、充值、直推收益等）。近期核对结论：

| 项 | 状态 |
|----|------|
| 管理端 + 用户端主路径 API 冒烟 | 通过（见下方脚本） |
| 管理端写操作（VIP / 锁定 / 停分红）沙箱冒烟 | 通过，测完还原 |
| **dapp-admin** 侧栏 | 本仓 `my_auth_list` / 外部 `myRouter.js` 已是主路径菜单，本轮未改 |
| **taurus** 导航收口 | 在外部仓 `taurus`：侧栏/底栏仅主路径；未接路由（商城/转账/质押等）重定向首页，`/pledge` → `/node` |

本期明确不做的前端能力（兑换、多商城、管理员权限 CRUD 等）不接后端、不进主菜单。

## 冒烟

前置：MySQL 已导入 schema，服务已在 `:8000` 启动（`make run` 或 `.\bin\app.exe -conf ...`）。

有 GNU Make 时：

```bash
make test          # biz 单测
make smoke         # 主路径读写（先检查 /health）
make smoke-admin   # 管理端写操作沙箱（VIP/锁定/停分红，测完还原）
make seed-demo     # 灌演示用户树（直连 MySQL）
```

Windows 无 make 时：

```powershell
.\scripts\regress.ps1 -Test
.\scripts\regress.ps1 -Smoke
.\scripts\regress.ps1 -SmokeAdmin
.\scripts\regress.ps1 -SeedDemo
.\scripts\regress.ps1 -All
```

演示步骤见 [`docs/demo.md`](docs/demo.md)。

等价直跑：

```bash
go run scripts/smoke_mainpath.go
go run scripts/smoke_admin_write.go
go run scripts/seed_demo_users.go
```

手工 curl 示例见 [`scripts/smoke_p0.md`](scripts/smoke_p0.md)。

## 目录

- `scripts/schema.sql` — 表结构与种子数据
- `scripts/smoke_mainpath.go` — 主路径冒烟（含 `/health` db、`settle_status`、提取打款关断言）
- `scripts/smoke_admin_write.go` — 管理端写操作沙箱冒烟
- `scripts/regress.ps1` — Windows 回归/演示入口（无 make）
- `docs/demo.md` — 演示手册（种子 + 管理端/用户端看点）
- `app/app/internal/biz` — 业务逻辑（rate/generation/community/settle）
- `app/app/internal/data` — GORM Repo
- `app/app/internal/service/compat.go` — 前端兼容 HTTP 处理器
- `CONTEXT.md` — 领域术语

## 测试

```bash
make test && make smoke && make smoke-admin
# 或
.\scripts\regress.ps1 -All
```
