-- ADR 0018: extract rate turn uses last static settle rate as base
ALTER TABLE locations
    ADD COLUMN last_settled_rate DECIMAL(5, 2) NULL
        COMMENT 'rate used in last static settle; extract turn base'
        AFTER rate_turn_pending;
