# 生产提取打款运维清单

对齐 [ADR 0010](adr/0010-withdraw-onchain-payout.md)。默认 `payout_enabled: false`，上线前按本清单核对。

## 上线前

1. **数据库迁移**：执行 `scripts/migrate_withdraw_payout.sql`（`tx_hash` / `payout_error` / 状态语义）。
2. **热钱包**：准备 BSC 热钱包；私钥仅用环境变量 `JINNIU_HOT_WALLET_KEY`，勿写入仓库或聊天。
3. **RPC / USDT**：`JINNIU_BSC_RPC`（或配置 `bsc_rpc`）、配置中 `usdt_address` 指向主网 USDT。
4. **单笔上限（强制）**：开打款时必须设 `JINNIU_PAYOUT_MAX_USDT>0`，否则**进程拒绝启动**；打款时仍会再校验，超额 → `BIZ_PAYOUT_ABOVE_MAX`。
5. **强制日结**：生产 `allow_force_settle: false`（启动时若为 true 会打 WARN）。
6. **定时打款（可选）**：`app.payout_cron` 非空则进程内定时扫队列；空则仅管理端「打款 / 跑队列」。
7. **开启打款**：确认热钱包有足够 USDT + 少量 BNB 气费后，再设 `JINNIU_PAYOUT_ENABLED=1` + `JINNIU_PAYOUT_MAX_USDT=…` 并重启。

## 可观测

| 入口 | 说明 |
|------|------|
| `GET /api/admin_jinniu/payout_status` | `enabled` / `key_configured` / `hot_address` / `max_usdt` / `max_required_ok` / `payout_cron` / `queue_rewarded` / `queue_doing` / `allow_force_settle`（**不查链上余额**） |
| `GET /api/admin_jinniu/all` | 同上字段，前缀 `payout_*`，管理端首页提示条 |
| 提取列表 | 单笔「打款」「确认」；队列 `rewarded`→`doing`→`pass` |

热钱包地址由私钥推导；链上余额请用区块浏览器或外部监控，本服务不拉余额。

## 日常操作

1. 用户申请提取 → 审核通过（`rewarded`，可提掉头）→ 打款占单 `doing` → 广播后写 `tx_hash` → 确认回执 → `pass`。
2. 进程崩溃在广播后：有 `tx_hash` 的 `doing` 单用「确认」或再点打款走确认路径，避免双花。
3. 打款关闭时：审核仍可过；管理端打款返回 `BIZ_PAYOUT_DISABLED`。
4. 队列积压：看首页提示或 `payout_status`；可 `POST /withdraw_payout_run` 批量处理。

## 主网小额联调（B1）

本机 PowerShell（**勿把私钥贴进聊天或提交 git**）：

```powershell
$env:JINNIU_PAYOUT_ENABLED = "1"
$env:JINNIU_HOT_WALLET_KEY = "<本机热钱包私钥hex>"
$env:JINNIU_PAYOUT_MAX_USDT = "1"   # 到账额 >1 拒绝
# 可选: $env:JINNIU_BSC_RPC = "https://bsc-dataseed.binance.org/"
.\bin\app.exe -conf app/app/configs/config.yaml
```

建议：审核一笔到账额 ≤1 USDT 的 `rewarded` → 管理端「打款」→ 浏览器核对 `tx_hash` → 「确认」→ `pass`。热钱包需有 USDT + BNB。

## 回滚 / 应急

- 立即关掉打款：去掉 `JINNIU_PAYOUT_ENABLED` 或设 `0` 后重启；已 `doing`+`tx_hash` 仍可确认，不再新占单广播。
- 私钥泄露：停打款、迁移热钱包资金、轮换密钥与配置。

## 本地联调（勿用生产钥）

见仓库 [README.md](../README.md)「本地联调打款」小节。
