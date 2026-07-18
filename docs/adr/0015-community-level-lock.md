# ADR 0015: 管理端手设社区等级并锁定

手设级别后结算仍按业绩重算会盖掉运维意图。决定：`vip_update` 写入级别并置 `community_level_locked`；结算时若锁定则保留级别、只更新 `community_volume`；另提供 `vip_unlock` 解除锁定，之后按业绩重算。
