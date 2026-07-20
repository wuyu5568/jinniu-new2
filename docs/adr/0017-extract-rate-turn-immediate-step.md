# ADR 0017: 提取利率掉头立刻步进；down / 最低档 up 不掉头

**Status**: superseded by ADR 0018

产品改为：提取确认时，对进行中且方向为 **up**、且利率高于最低档的认购单，先翻成 down 再立刻 `AdvanceRate` 一档写入新利率，下次结算按该新档计息；方向为 **down** 不掉头；**最低档且 up** 不掉头（避免触底反弹加档）。同周期仍用 `rate_turn_pending` 防重复。废止 ADR 0013「只翻方向、下次仍按旧档计息」。

已被 ADR 0018 废止：现改为以 `last_settled_rate`（昨日结算利率）为掉头起点。
