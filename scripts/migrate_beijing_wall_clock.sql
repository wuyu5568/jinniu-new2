-- Convert DATETIME wall-clock UTC → Beijing (+8h). Idempotent via schema_migrations.
-- Does NOT touch DATE columns (e.g. settle_runs.settle_date) or TIMESTAMP columns.

CREATE TABLE IF NOT EXISTS schema_migrations (
    id         VARCHAR(64)  NOT NULL PRIMARY KEY,
    applied_at DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    remark     VARCHAR(255) NOT NULL DEFAULT ''
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Guard: abort if already applied (checked by deploy script before sourcing).
-- Manual re-run protection:
-- INSERT will fail if id exists when deploy script inserts after success.

UPDATE users
SET disabled_at = IF(disabled_at IS NULL, NULL, disabled_at + INTERVAL 8 HOUR),
    created_at  = created_at + INTERVAL 8 HOUR,
    updated_at  = updated_at + INTERVAL 8 HOUR;

UPDATE login_challenges
SET expires_at = expires_at + INTERVAL 8 HOUR,
    used_at    = IF(used_at IS NULL, NULL, used_at + INTERVAL 8 HOUR),
    created_at = created_at + INTERVAL 8 HOUR;

UPDATE locations
SET created_at = created_at + INTERVAL 8 HOUR,
    updated_at = updated_at + INTERVAL 8 HOUR;

UPDATE user_recommends
SET created_at = created_at + INTERVAL 8 HOUR,
    updated_at = updated_at + INTERVAL 8 HOUR;

UPDATE ledger_entries
SET created_at = created_at + INTERVAL 8 HOUR;

UPDATE withdraws
SET reviewed_at = IF(reviewed_at IS NULL, NULL, reviewed_at + INTERVAL 8 HOUR),
    created_at  = created_at + INTERVAL 8 HOUR,
    updated_at  = updated_at + INTERVAL 8 HOUR;

UPDATE eth_user_record
SET created_at = created_at + INTERVAL 8 HOUR,
    updated_at = updated_at + INTERVAL 8 HOUR;

UPDATE settle_runs
SET created_at = created_at + INTERVAL 8 HOUR,
    updated_at = updated_at + INTERVAL 8 HOUR;
-- settle_date (DATE) intentionally unchanged

INSERT INTO schema_migrations (id, remark)
VALUES ('beijing_wall_clock_v1', 'DATETIME +8h; OS/app Asia/Shanghai; TIMESTAMP via session +08:00');
