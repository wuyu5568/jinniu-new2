# ADR 0018: 提取利率掉头以昨日结算利率为起点

**Status**: accepted（supersedes ADR 0017）

提取确认时不再从订单「日结后已步进」的当前利率起步，而以订单字段 `last_settled_rate`（每次静态计息写入的昨日结算利率）为起点：方向 down 或字段为空则不掉头；昨日已是最低档则改回最低档 / up（今天仍按最低档计息）；否则对该昨日档 `AdvanceRate(down)` 一档。同周期仍用 `rate_turn_pending` 防重复。
