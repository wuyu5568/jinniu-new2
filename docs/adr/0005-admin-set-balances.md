# ADR 0005: 管理端修改余额为设目标值

## Status

Accepted

## Context

用户列表原「增加可提 / 充值账户」只能正向加款，运维纠错（下调、清零）不便。与侧边栏「账户余额充值」共用 `add_money_three` 时，若直接改语义会破坏加款页。

## Decision

- `add_money_two` / `add_money_three`：将可提 / 账户余额设为绝对目标值（≥0）；差额写账本。
- 账户差额：`deposit`；可提差额：`admin_adjust`（不进收益、不进充值列表）。
- 侧边栏充值页改走 `POST /deposit`，仍只增加账户余额。

## Consequences

- 旧脚本若按「加一笔」调用 `add_money_*` 会变成「设成该数」，需改为目标值或改用 `/deposit`。
- 目标值与当前相同时不写账本。
