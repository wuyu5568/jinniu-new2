-- 日结防重（上海自然日唯一）
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
