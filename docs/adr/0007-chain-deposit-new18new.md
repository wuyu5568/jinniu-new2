# ADR 0007: 链上充值对齐 new18new 合约序号拉取

## Status

Accepted（已实现；2026-07-15 **C2 加固**；**C2+** 单次 chunk=50/~90s，`GET /deposit?until_caught_up=1` 多轮追上，上限 5min/500 序号，回执含 `caught_up`）

## Context

CONTEXT 曾写 USDT Transfer 扫块；产品要求按 new18new：专用充值合约 + 序号游标 + 管理端 HTTP 触发。

早期实现存在：`<100` 提前结束整批、未注册静默跳过却可能把 `last` 写成整段 `userLength`、接口几乎无回执等问题。

## Decision

- BSC 主网合约 `0x49c735D94e1cc44053D23c956972cB37da3Fd5Af`（BuySomething）；RPC 多节点轮询。
- `GET /api/admin_jinniu/deposit` 拉新充值；进程内定时见 **ADR 0027**（本 ADR 原稿曾写不加 cron，已废止）。
- 表 `eth_user_record`：**一合约序号一行**，`last` = 该序号，**`UNIQUE(last)`** 幂等。
- 游标：`MAX(last)`；空表视为 `-1`；下次从 `last+1` 处理到 `userLength-1`。
- 金额 **≥ 5** 且地址已注册 → `status=success`，入账户余额并写 `ledger_entries` deposit（见 ADR 0028；原 ≥100 / 无最低额已废止）。
- 未注册或 **< 5** → **仍写行并推进**（C1）：`status=skipped`，`user_id=0`（未注册），地址放 `hash`，`type` 记原因（`unregistered` / `below_min`）。
- 回执（A1）：`pulled` / `credited` / `skipped` / `errors` + `cursor_before` / `cursor_after` + `skip_reasons` 聚合。
- 管理端 `POST /deposit` 仍为人工增加账户余额。

## Consequences

- 未注册用户链上充值会被跳过且不挡后续；补注册后可用 `POST /api/admin_jinniu/deposit_replay`（管理端「按序号补账」）重放 `skipped` 行：链上金额为权威，仅 `status=skipped` 可转 `success`，已入账不会双记。
- 历史错误 `last=userLength` 多行需迁移去重后再加 UNIQUE；去重后游标可能偏高，漏序号需运维评估。
- 换合约需改硬编码与前端 `VITE_BUY`。
