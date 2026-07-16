# ADR 0010: 提取资产链上打款（设计：审核与打款分离）

## Status

Accepted（2026-07-15）；**已实现**（本地默认 `payout_enabled: false`）。联调可用环境变量 `JINNIU_PAYOUT_ENABLED` / `JINNIU_HOT_WALLET_KEY` 覆盖，勿把私钥写入仓库。生产开启前须确认热钱包 USDT/BNB 余额。

## Context

金牛此前约定「本期不做链上打款」：管理端通过提取只做账本确认 + 利率掉头。要对齐 new18new 的出金能力，且避免「点通过即上链」把审核与 RPC/私钥故障绑死。

## Decision

1. **结构（S1）**：审核与打款拆开。通过 = 账本终态确认并进入待打队列；链上转账由独立执行器完成。
2. **状态字（W2，对齐 new18new 库内用词）**  
   - `pending`：待审（申请时已扣可提、锁定手续费与到账额）  
   - `rewarded`：已审待打（取代原「通过即 `approved`」的终态语义）  
   - `doing`：打款中（已占单，可有/将有 `tx_hash`）  
   - `pass`：链上成功  
   - `rejected` / `cancelled`：拒审/用户取消（退回可提）  
   历史数据中的 `approved`：实现时迁移或展示映射为 `rewarded`。
3. **签名（K1）**：服务端热钱包在 **BSC** 转 **USDT**（合约地址配置化，默认主网 USDT）；私钥仅配置注入，不进仓库。
4. **防双花（F1）**：以 `withdraw.id` 为幂等键。`rewarded`→`doing` 原子占单；广播后**先持久化 `tx_hash`**；确认回执后再 `pass`。重试时若已有 hash 则只查链确认，禁止再发一笔。
5. **触发（J3）**：进程内 cron 扫 `rewarded`/`doing`；管理端提供单笔/批量打款与「确认回执」。同 ID 全局互斥。
6. **金额**：打 `credited_amount`（到账额）。打款失败**不**自动退可提（申请时已扣）；仅重试打款或人工运维。
7. **与充值**：入金仍走既有 BuySomething 游标同步（ADR 0007）；本 ADR 只覆盖**出金**。

## Consequences

- 实现需：`withdraws` 增加 `tx_hash`（及可选 `payout_error`）、配置项（RPC、USDT、私钥、打款 cron）、审核通过写 `rewarded`、打款 worker、管理端文案/按钮、迁移旧 `approved`。
- 热钱包权限与余额监控成为运维硬要求；`allow` 类开关应能关闭自动打款。
- 领域上「提取资产」不再等于「仅账本」；通过 ≠ 用户已收到链上 USDT。
