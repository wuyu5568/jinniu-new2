# ADR 0027: 链上充值进程内每分钟自动拉取

## Status

Accepted（2026-07-22）

## Context

ADR 0007 明确「本轮不加 cron」，入账依赖管理端 `GET /api/admin_jinniu/deposit`。产品要求用户 `buy` 后约一分钟内自动入账，不能依赖人工点按钮。

## Decision

- 配置 `app.deposit_cron`（默认 `*/1 * * * *`）；空字符串则关闭定时，仍可手动拉。
- 进程内 `DepositCron`（`robfig/cron`）每触发一次跑**单轮** `syncChainDeposits`（chunk=50、墙钟约 90s），与手动接口共用同一套入账逻辑。
- 上一轮未结束则跳过本轮（`atomic` 忙锁），避免重叠打 RPC。
- 多实例重复拉由 `eth_user_record UNIQUE(last)` 幂等消化；不另做分布式锁。

## Consequences

- 正常积压下约一分钟内入账；积压超过单轮容量时靠后续分钟追赶，或管理端 `until_caught_up=1`。
- 运维须在生产 `/etc/jinniu/config.yaml` 写入 `deposit_cron` 并重启 `jinniu.service`。
- ADR 0007 中「无进程内定时」被本 ADR 取代；合约序号与入账规则不变。
