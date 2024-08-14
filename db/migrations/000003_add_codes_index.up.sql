CREATE INDEX idx_code_usage_code_batch_client_customer
ON code_usage (code, batch_id, client_id, customer_id);
