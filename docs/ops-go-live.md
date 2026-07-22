# 完整上线清单（含链上打款）

目标：生产环境跑通 **认购 / 日结 / 链上充值 / 提取审核 / 热钱包打款**。  
打款细则另见 [ops-payout.md](ops-payout.md)。

**原则**：密钥与私钥不进 git、不进聊天；配置用服务器上的 `config.prod.yaml` + 环境变量。

---

## 阶段 0：代码入库

本机（只需做一次）：

```powershell
git config --global user.name "你的名字"
git config --global user.email "你的邮箱"
```

在 `new2` 仓库：

```powershell
cd C:\Users\Lenovo\Desktop\github\new2
git add -A
git status   # 确认无真实私钥、无 .env
git commit -m "feat: jinniu production-ready backend"
# 若有远程：git push -u origin HEAD
```

`dapp-admin` / `taurus` 同样各自 commit（前端对接改动）。  
**之后部署只拉某次 commit / 某 tag，不要从脏工作区直接拷。**

---

## 阶段 1：生产 MySQL

1. 建库（名称自定，下文用 `jinniu`）：

```sql
CREATE DATABASE IF NOT EXISTS jinniu DEFAULT CHARSET utf8mb4;
```

2. **新库**：导入全量结构 + 默认参数：

```bash
mysql -h<HOST> -u<USER> -p jinniu < scripts/schema.sql
# 可选中文配置名
mysql -h<HOST> -u<USER> -p jinniu < scripts/seed_config_names_zh.sql
```

3. **若库是早期半成品**（已有表但缺补丁），按需执行（可重复前先看脚本注释）：

| 脚本 | 作用 |
|------|------|
| `scripts/migrate_eth_user_record.sql` | 链上充值记录表（若 schema 已含可跳过） |
| `scripts/migrate_eth_deposit_cursor.sql` | `UNIQUE(last)` 游标幂等 |
| `scripts/migrate_settle_runs.sql` | 日结防重 |
| `scripts/migrate_withdraw_payout.sql` | 提取打款字段 / 状态 |

4. 抽查：`users`、`locations`、`withdraws`、`eth_user_record`、`settle_runs`、`business_configs` 存在；`eth_user_record.last` 有唯一索引。

---

## 阶段 2：生产配置文件（不要用仓库里的本地 yaml 原样上线）

在服务器单独放一份配置（例：`/etc/jinniu/config.yaml`），相对本地至少改：

| 项 | 生产值 |
|----|--------|
| `data.database.source` | 生产 DSN（强密码、限网段） |
| `auth.jwt_key` | 长随机串（≥32 字节） |
| `auth.admin_username` / `admin_password` | 强密码；勿用 `admin123` |
| `app.genesis_address` | 正式创世钱包（小写） |
| `app.allow_force_settle` | **`false`** |
| `app.payout_enabled` | yaml 里保持 **`false`**（用环境变量开，见阶段 4） |
| `app.hot_wallet_key` | **留空**（只用环境变量） |
| `app.bsc_rpc` / `usdt_address` | BSC 主网 RPC；USDT `0x55d398…7955`（与现网一致即可） |
| `app.settle_cron` / `settle_timezone` | 如 `0 0 * * *` + `Asia/Shanghai` |
| `app.payout_cron` | 可选；空 = 仅管理端点打款 |
| `app.deposit_cron` | 链上充值自动拉取；默认 `*/1 * * * *`；空 = 仅管理端点 |

---

## 阶段 3：先「软上线」（打款仍关）

1. 编译并部署后端二进制 + 配置。
2. **先不要**设 `JINNIU_PAYOUT_ENABLED`。
3. 启动后检查：

```bash
curl -s https://<你的域名或IP>:8000/health
# 期望 {"status":"ok","db":"ok"}
```

4. 部署 `dapp-admin` / `taurus`，API 基址指向生产后端。
5. 验收（约 15 分钟）：
   - 管理端登录（新密码）
   - 首页：日结提示正常；打款提示应为「已关闭」
   - 用户端：注册/登录、管理充值或拉链上充值、认购、申请提取、审核通过 → 状态 `rewarded`
   - 点「打款」应失败且提示打款关闭（`BIZ_PAYOUT_DISABLED`）——说明审核与打款已分离，符合预期
   - 手动日结一次；再点应 `already_settled`，且**没有**「强制再跑」

软上线通过后，再进入阶段 4。

---

## 阶段 4：打开完整打款（B1 → 生产额度）

### 4.1 准备热钱包

- 新建专用 BSC 热钱包（勿用个人常用钱包）。
- 转入：足够应付队列的 **USDT** + 少量 **BNB**（gas）。
- 私钥只放服务器环境变量，**永不**写进 yaml / git / 聊天。

### 4.2 先小额验证（强烈建议）

停服务 → 设环境变量 → 再启：

```powershell
# Windows 服务机示例；Linux 用 export
$env:JINNIU_PAYOUT_ENABLED = "1"
$env:JINNIU_HOT_WALLET_KEY = "<私钥hex>"
$env:JINNIU_PAYOUT_MAX_USDT = "1"
# 可选: $env:JINNIU_BSC_RPC = "https://bsc-dataseed.binance.org/"
```

然后：

1. 管理端 `payout_status` / 首页：`enabled=true`，`key_configured=true`，显示热钱包地址，与浏览器核对。
2. 人为造一笔**到账额 ≤ 1 USDT** 的 `rewarded` → 「打款」→ 浏览器看 `tx_hash` → 「确认」→ `pass`。
3. 再试一笔到账额 **> 1** → 应 `BIZ_PAYOUT_ABOVE_MAX`（证明上限生效）。

### 4.3 调到生产额度并可选定时

```powershell
$env:JINNIU_PAYOUT_MAX_USDT = "500"   # 按你们风控改数字
# yaml 里可设 payout_cron，例如每 5 分钟："*/5 * * * *"
```

重启后处理积压：`POST /api/admin_jinniu/withdraw_payout_run` 或管理端批量打款。

---

## 阶段 5：上线后日常

| 事项 | 怎么做 |
|------|--------|
| 健康 | 监控 `/health`，`db` 必须 `ok` |
| 日结 | 看首页 / `settle_status`；失败人工重跑（勿开 force） |
| 充值积压 | 「拉链上充值」或「拉到追上」；跳过补账用「按序号补账」 |
| 打款积压 | 首页 `queue_rewarded` / `queue_doing`；热钱包余额用浏览器看 |
| 应急关打款 | `JINNIU_PAYOUT_ENABLED=0` 重启；已广播的 `doing` 仍可「确认」 |

---

## 完成标准（勾选）

- [ ] 代码已 commit（及可选 push）
- [ ] 生产库 schema + 必要 migrate 已执行
- [ ] jwt / 管理密码 / DSN / genesis 已换生产值
- [ ] `allow_force_settle: false`
- [ ] 软上线验收通过（打款关）
- [ ] 热钱包小额打通（max=1）
- [ ] 生产 `JINNIU_PAYOUT_MAX_USDT` 已设合理顶
- [ ] `payout_status` 正常；首页提示与队列可读
- [ ] 应急关打款步骤已演练或写进值班文档

全部勾完即可视为 **完整版上线完成**。
