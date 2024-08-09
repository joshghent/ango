-- Remove the 'name' column from the 'batches' table
ALTER TABLE batches
DROP COLUMN name;

-- Re-add the 'used_at' column to the 'codes' table
ALTER TABLE codes
ADD COLUMN used_at TIMESTAMP;
