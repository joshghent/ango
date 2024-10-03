-- Remove the default name from existing records
UPDATE batches SET name = '' WHERE name = 'Unnamed Batch';

-- Remove the added columns
ALTER TABLE batches
DROP COLUMN expired,
DROP COLUMN name,
ADD COLUMN name TEXT;
