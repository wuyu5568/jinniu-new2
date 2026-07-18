-- ADR 0015: admin-set community level lock (settle updates volume only)
ALTER TABLE users
    ADD COLUMN community_level_locked TINYINT(1) NOT NULL DEFAULT 0
        COMMENT '1=admin-set level; settle only updates volume'
        AFTER community_volume;
