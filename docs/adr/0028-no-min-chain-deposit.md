# ADR 0028: 链上充值最低金额

## Status

Accepted（2026-07-22：曾取消最低额；同日改为 **最低 5 USDT**）

## Context

ADR 0007 原为 ≥100；产品一度取消门槛；现要求最低 **5 USDT**。

## Decision

- 入账条件：**已注册且 amount ≥ 5**；`< 5` 记 `skipped/below_min` 并推进游标。
- 补账 `deposit_replay`：`< 5` 仍为 `still_skipped`；≥5 且已注册可补账。
- 用户端输入与提示同步最低 5。

## Consequences

- 低于 5 的链上 `buy` 不会入账账户余额（仍记 skipped）。
- 取代「无最低额」口径；原 ≥100 仍废止。
