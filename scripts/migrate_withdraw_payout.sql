-- ADR 0010: withdraw on-chain payout columns + status comment
ALTER TABLE withdraws
    ADD COLUMN tx_hash VARCHAR(80) NOT NULL DEFAULT '' COMMENT 'BSC tx hash after broadcast' AFTER remark,
    ADD COLUMN payout_error VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'last payout error' AFTER tx_hash;

-- Legacy approved (=已审) → rewarded (待打款)
UPDATE withdraws SET status = 'rewarded' WHERE status = 'approved';
