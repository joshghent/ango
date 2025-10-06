-- Add used_at column back to codes table for tracking when codes were redeemed
ALTER TABLE codes ADD COLUMN used_at TIMESTAMP;