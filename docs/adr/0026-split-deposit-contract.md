# 对接分账充值合约

## Status

Accepted（地址已换为 `0x0f2994708Ecf85b98AAc9f5aAD4d0a036f197999`，2026-07-22）

## Context

产品要求链上充值即时按固定比例转到多个收款地址；接口形态为 `buy(num)`、`users`/`usersAmount` 序号、`getUsersByIndex` / `getUsersAmountByIndex` / `getUserLength`。线上地址先后为 `0xb8f765…` → `0x1E13586a…` → **`0x0f2994708Ecf85b98AAc9f5aAD4d0a036f197999`**。

## Decision

- 金牛后端 `chainDepositContract` 指向当前线上地址；拉取仍走 ADR 0007 游标与 skipped/补账，**一序号一行**，不采用 `Last=userLength`。
- `usersAmount` 存整数 `num`（USDT）；`amountAsUSDT` 兼容 wei 与整数。
- 合约无 `ids`：`GetIdsByIndex` 失败则忽略，`Id=0`。
- 前端 `VITE_BUY` 指向同一合约；`buy(num)` 单参数（ABI 同步）。
- 入账仍按用户充值全额记账户余额；分账仅链上资金去向。
- **换约**后游标与旧合约序号空间无关：清空 `eth_user_record` 或确认从新约序号 0 起拉。

## Consequences

- 旧合约不再作为入口。
- 清游标后首次拉链从新合约序号 0 起；未注册记 skipped。
- 分账比例与收款地址以当前链上合约为准。
