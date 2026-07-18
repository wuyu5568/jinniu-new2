-- 金牛协议 Phase 1 schema（勿用 GORM AutoMigrate，以本文件为准）
-- CREATE DATABASE jinniu DEFAULT CHARSET utf8mb4;

CREATE TABLE IF NOT EXISTS users (
    id                   BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    address              VARCHAR(64)     NOT NULL COMMENT 'wallet address lowercase',
    inviter_id           BIGINT UNSIGNED NULL COMMENT 'inviter user id, null for genesis',
    account_balance      DECIMAL(36, 8)  NOT NULL DEFAULT 0 COMMENT 'account balance for subscribe',
    withdrawable_balance DECIMAL(36, 8)  NOT NULL DEFAULT 0 COMMENT 'withdrawable balance',
    community_level      TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '0=none, 1-9 = V1-V9',
    community_volume     DECIMAL(36, 8)  NOT NULL DEFAULT 0 COMMENT 'community volume snapshot',
    community_level_locked TINYINT(1)    NOT NULL DEFAULT 0 COMMENT '1=admin-set level; settle only updates volume',
    disabled_at          DATETIME(3)     NULL COMMENT 'soft delete / disabled',
    reward_locked        TINYINT(1)      NOT NULL DEFAULT 0 COMMENT '1=stop as reward source for uplines',
    created_at           DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at           DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    UNIQUE KEY uk_users_address (address),
    KEY idx_users_inviter (inviter_id),
    KEY idx_users_community_level (community_level),
    KEY idx_users_disabled (disabled_at),
    CONSTRAINT fk_users_inviter FOREIGN KEY (inviter_id) REFERENCES users (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS login_challenges (
    id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    address     VARCHAR(64)     NOT NULL,
    nonce       VARCHAR(64)     NOT NULL,
    expires_at  DATETIME(3)     NOT NULL,
    used_at     DATETIME(3)     NULL,
    created_at  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    UNIQUE KEY uk_login_challenges_nonce (nonce),
    KEY idx_login_challenges_address (address)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- locations = 认购订单; status: active | exited; rate_direction: up | down
-- rate_turn_pending: extract flipped direction once this settle cycle; cleared on next settle
CREATE TABLE IF NOT EXISTS locations (
    id                 BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id            BIGINT UNSIGNED NOT NULL,
    amount             DECIMAL(36, 8)  NOT NULL,
    multiplier         DECIMAL(3, 1)   NOT NULL,
    exit_target        DECIMAL(36, 8)  NOT NULL,
    accumulated        DECIMAL(36, 8)  NOT NULL DEFAULT 0,
    status             VARCHAR(16)     NOT NULL DEFAULT 'active',
    rate_percent       DECIMAL(5, 2)   NOT NULL DEFAULT 0.60,
    rate_direction     VARCHAR(8)      NOT NULL DEFAULT 'up',
    rate_turn_pending  TINYINT(1)      NOT NULL DEFAULT 0,
    created_at         DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at         DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    KEY idx_locations_user (user_id),
    KEY idx_locations_status (status),
    KEY idx_locations_user_status (user_id, status),
    CONSTRAINT fk_locations_user FOREIGN KEY (user_id) REFERENCES users (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS user_recommends (
    id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id    BIGINT UNSIGNED NOT NULL,
    path       VARCHAR(2048)   NOT NULL DEFAULT '' COMMENT 'ancestor ids e.g. 1,5,12',
    created_at DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    UNIQUE KEY uk_user_recommends_user (user_id),
    KEY idx_user_recommends_path (path(255)),
    CONSTRAINT fk_user_recommends_user FOREIGN KEY (user_id) REFERENCES users (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS ledger_entries (
    id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id      BIGINT UNSIGNED NOT NULL,
    order_id     BIGINT UNSIGNED NULL COMMENT 'location id when applicable',
    entry_type   VARCHAR(32)     NOT NULL,
    amount       DECIMAL(36, 8)  NOT NULL,
    balance_kind VARCHAR(16)     NOT NULL COMMENT 'account | withdrawable',
    remark       VARCHAR(255)    NOT NULL DEFAULT '',
    created_at   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    KEY idx_ledger_user (user_id),
    KEY idx_ledger_order (order_id),
    KEY idx_ledger_type (entry_type),
    KEY idx_ledger_created (created_at),
    KEY idx_ledger_user_created (user_id, created_at),
    CONSTRAINT fk_ledger_user FOREIGN KEY (user_id) REFERENCES users (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS withdraws (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id         BIGINT UNSIGNED NOT NULL,
    amount          DECIMAL(36, 8)  NOT NULL COMMENT 'requested extract amount',
    fee_amount      DECIMAL(36, 8)  NOT NULL COMMENT 'fee locked at apply',
    credited_amount DECIMAL(36, 8)  NOT NULL COMMENT 'net credited at apply',
    order_ids_json  JSON            NOT NULL COMMENT 'location ids for reversal',
    status          VARCHAR(16)     NOT NULL DEFAULT 'pending' COMMENT 'pending|rewarded|doing|pass|rejected|cancelled',
    remark          VARCHAR(255)    NOT NULL DEFAULT '',
    tx_hash         VARCHAR(80)     NOT NULL DEFAULT '' COMMENT 'BSC tx hash after broadcast',
    payout_error    VARCHAR(255)    NOT NULL DEFAULT '' COMMENT 'last payout error',
    reviewed_at     DATETIME(3)     NULL,
    created_at      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    KEY idx_withdraws_user (user_id),
    KEY idx_withdraws_status (status),
    KEY idx_withdraws_created (created_at),
    KEY idx_withdraws_status_id (status, id),
    CONSTRAINT fk_withdraws_user FOREIGN KEY (user_id) REFERENCES users (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS business_configs (
    id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    config_key VARCHAR(64)     NOT NULL,
    name       VARCHAR(128)    NOT NULL,
    value      TEXT            NOT NULL,
    sort_order INT             NOT NULL DEFAULT 0,
    updated_at TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_config_key (config_key),
    KEY idx_sort (sort_order, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT INTO business_configs (config_key, name, value, sort_order) VALUES
('extract_fee_rate', '提取手续费比例', '0.06', 10),
('generation_rate', '代数奖比例', '0.05', 20),
('max_generation_depth', '最大代数深度', '10', 30),
('peer_pool_rate', '平级奖比例', '0.10', 40),
('min_peer_level', '平级最低等级(V)', '3', 50),
('rate_min', '静态利率下限(%)', '0.60', 60),
('rate_max', '静态利率上限(%)', '1.40', 70),
('rate_step', '静态利率步进', '0.05', 80),
('min_subscribe_amount', '最低认购金额', '100', 90),
('min_withdraw_amount', '最低提取金额', '10', 91),
('subscribe_tiers', '认购档位(逗号分隔)', '100,500,1000,3000', 92),
('multiplier_1_lt', '出局倍数档1上限', '1000', 100),
('multiplier_1_multiplier', '出局倍数档1倍数', '2', 101),
('multiplier_2_lt', '出局倍数档2上限', '3000', 102),
('multiplier_2_multiplier', '出局倍数档2倍数', '2.5', 103),
('multiplier_3_lt', '出局倍数档3上限', '', 104),
('multiplier_3_multiplier', '出局倍数档3倍数', '3', 105),
('community_v9_min_volume', 'V9业绩门槛', '20000000', 200),
('community_v9_rate', 'V9社区基础奖比例', '0.60', 201),
('community_v8_min_volume', 'V8业绩门槛', '10000000', 202),
('community_v8_rate', 'V8社区基础奖比例', '0.55', 203),
('community_v7_min_volume', 'V7业绩门槛', '5000000', 204),
('community_v7_rate', 'V7社区基础奖比例', '0.50', 205),
('community_v6_min_volume', 'V6业绩门槛', '1500000', 206),
('community_v6_rate', 'V6社区基础奖比例', '0.45', 207),
('community_v5_min_volume', 'V5业绩门槛', '500000', 208),
('community_v5_rate', 'V5社区基础奖比例', '0.40', 209),
('community_v4_min_volume', 'V4业绩门槛', '250000', 210),
('community_v4_rate', 'V4社区基础奖比例', '0.35', 211),
('community_v3_min_volume', 'V3业绩门槛', '80000', 212),
('community_v3_rate', 'V3社区基础奖比例', '0.30', 213),
('community_v2_min_volume', 'V2业绩门槛', '20000', 214),
('community_v2_rate', 'V2社区基础奖比例', '0.20', 215),
('community_v1_min_volume', 'V1业绩门槛', '5000', 216),
('community_v1_rate', 'V1社区基础奖比例', '0.10', 217)
ON DUPLICATE KEY UPDATE name = VALUES(name), value = VALUES(value), sort_order = VALUES(sort_order);
-- 链上充值游标/记录（对齐 new18new eth_user_record）
CREATE TABLE IF NOT EXISTS eth_user_record (
    id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    hash       VARCHAR(100)    NOT NULL DEFAULT '',
    user_id    BIGINT UNSIGNED NOT NULL,
    status     VARCHAR(45)     NOT NULL DEFAULT 'success',
    type       VARCHAR(45)     NOT NULL DEFAULT 'deposit',
    amount     VARCHAR(64)     NOT NULL DEFAULT '',
    amount_two BIGINT UNSIGNED NOT NULL DEFAULT 0,
    coin_type  VARCHAR(45)     NOT NULL DEFAULT 'USDT',
    last       BIGINT          NOT NULL DEFAULT 0 COMMENT 'contract index cursor (unique per processed index)',
    created_at DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    UNIQUE KEY uk_eth_user_record_last (last),
    KEY idx_eth_user_record_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 日结防重
CREATE TABLE IF NOT EXISTS settle_runs (
    id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    settle_date    DATE            NOT NULL COMMENT 'Asia/Shanghai calendar day',
    forced         TINYINT(1)      NOT NULL DEFAULT 0,
    settled_count  INT             NOT NULL DEFAULT 0,
    exited_count   INT             NOT NULL DEFAULT 0,
    generation_count INT           NOT NULL DEFAULT 0,
    community_count INT            NOT NULL DEFAULT 0,
    peer_count     INT             NOT NULL DEFAULT 0,
    remark         VARCHAR(255)    NOT NULL DEFAULT '',
    created_at     DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at     DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    UNIQUE KEY uk_settle_runs_date (settle_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
