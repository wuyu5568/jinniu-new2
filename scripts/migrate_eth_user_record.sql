-- 增量：链上充值记录表（已有库执行一次）
CREATE TABLE IF NOT EXISTS eth_user_record (
    id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    hash       VARCHAR(100)    NOT NULL DEFAULT '',
    user_id    BIGINT UNSIGNED NOT NULL,
    status     VARCHAR(45)     NOT NULL DEFAULT 'success',
    type       VARCHAR(45)     NOT NULL DEFAULT 'deposit',
    amount     VARCHAR(64)     NOT NULL DEFAULT '',
    amount_two BIGINT UNSIGNED NOT NULL DEFAULT 0,
    coin_type  VARCHAR(45)     NOT NULL DEFAULT 'USDT',
    last       BIGINT          NOT NULL DEFAULT 0 COMMENT 'contract index cursor',
    created_at DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    KEY idx_eth_user_record_user (user_id),
    KEY idx_eth_user_record_last (last)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
