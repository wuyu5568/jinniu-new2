# ADR 0008: 日结同日防重 settle_runs

## Status

Accepted（2026-07-15 修订：L1 占位抢锁 + 限制 force）

## Context

日结重复执行会导致静态/代数/社区/平级重复入账。进程内 mutex 挡不住多实例。

## Decision

- 表 `settle_runs`，`settle_date`（Asia/Shanghai）唯一。
- **普通日结（cron / 手动无 force）**：先 `INSERT` 占位（TryClaim）；唯一键冲突 → `already_settled` 跳过；跑完再 Upsert 计数。占位成功但中途崩溃则当天不自动重跑（运维删行或测试环境 force）。
- **`force=1`**：仅当配置 `app.allow_force_settle=true` 时允许（仍可能双发，仅测试）；生产保持 `false`。
- 进程内 mutex 降低同进程并发。

## Consequences

- e2e 同日多次结算需 `force=1` 且本地打开 `allow_force_settle`。
- 多实例靠 INSERT 唯一键原子占日，不再依赖「先查再写」。
