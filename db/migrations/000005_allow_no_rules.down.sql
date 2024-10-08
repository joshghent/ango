-- Reverse operation: Disallow NULL values for the "rules" column in the batches table
ALTER TABLE batches ALTER COLUMN rules SET NOT NULL;
