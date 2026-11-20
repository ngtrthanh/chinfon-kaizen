-- Real reward register: a review approval cannot assert paid without an external
-- approved payout document. Historical pre-migration paid rows have no reference;
-- HR reconciliation must flag and resolve them before authoritative reporting.
ALTER TABLE initiative ADD COLUMN payment_reference TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_initiative_payment_ref ON initiative(payment_reference);
