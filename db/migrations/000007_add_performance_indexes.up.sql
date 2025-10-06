-- Critical index for unredeemed code lookup
CREATE INDEX IF NOT EXISTS idx_codes_batch_client_unredeemed
ON codes(batch_id, client_id, id) WHERE customer_id IS NULL;

-- Index for customer rule checking
CREATE INDEX IF NOT EXISTS idx_codes_customer_used
ON codes(customer_id, id) WHERE customer_id IS NOT NULL;

-- Index for batch management
CREATE INDEX IF NOT EXISTS idx_batches_expired
ON batches(expired) WHERE expired = false;

-- Index for pre-allocation queries
CREATE INDEX IF NOT EXISTS idx_codes_batch_client_total
ON codes(batch_id, client_id);

-- Partial index for available codes count
CREATE INDEX IF NOT EXISTS idx_codes_available_count
ON codes(batch_id, client_id) WHERE customer_id IS NULL;