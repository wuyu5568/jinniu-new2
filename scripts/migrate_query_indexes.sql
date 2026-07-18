-- Hot-path composite indexes for settle / list / payout queries
-- Safe to re-run only if you drop first; production runner checks existence.

-- locations: user orders by status (active settle / list)
ALTER TABLE locations
    ADD INDEX idx_locations_user_status (user_id, status);

-- ledger: user timeline
ALTER TABLE ledger_entries
    ADD INDEX idx_ledger_user_created (user_id, created_at);

-- withdraws: payout queue by status then id
ALTER TABLE withdraws
    ADD INDEX idx_withdraws_status_id (status, id);
