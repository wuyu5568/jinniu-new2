-- C2 / ADR 0007: one row per contract index; UNIQUE(last) for idempotent cursor
-- Dedupe existing rows that share the same last (keep highest id)
DELETE t1 FROM eth_user_record t1
INNER JOIN eth_user_record t2
  ON t1.last = t2.last AND t1.id < t2.id;

-- Add unique cursor index (ignore if already present)
ALTER TABLE eth_user_record
    ADD UNIQUE KEY uk_eth_user_record_last (last);
