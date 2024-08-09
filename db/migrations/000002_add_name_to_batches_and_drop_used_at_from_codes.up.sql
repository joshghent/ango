-- Add a 'name' column to the 'batches' table
ALTER TABLE batches
ADD COLUMN name TEXT;

-- Drop the 'used_at' column from the 'codes' table
ALTER TABLE codes
DROP COLUMN used_at;
