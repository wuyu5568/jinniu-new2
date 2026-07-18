-- ADR 0013: extract rate turn = flip direction only; flag cleared on next settle (pay current then advance)
ALTER TABLE locations
    ADD COLUMN rate_turn_pending TINYINT(1) NOT NULL DEFAULT 0
        COMMENT 'extract flipped direction once this settle cycle; cleared on next settle'
        AFTER rate_direction;
