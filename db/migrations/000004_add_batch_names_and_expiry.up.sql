ALTER TABLE batches
DROP COLUMN name,
ADD COLUMN name VARCHAR(255) NOT NULL DEFAULT '',
ADD COLUMN expired BOOLEAN NOT NULL DEFAULT FALSE;

-- Update existing records to have a default name if needed
UPDATE batches SET name = 'Unnamed Batch' WHERE name = '';
