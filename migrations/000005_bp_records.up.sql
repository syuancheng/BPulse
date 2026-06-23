CREATE TABLE bp_records (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  owner_id BIGINT UNSIGNED NOT NULL,
  client_request_id VARCHAR(80) NOT NULL,
  measured_at_utc DATETIME(6) NOT NULL,
  timezone VARCHAR(64) NOT NULL,
  entry_method VARCHAR(16) NOT NULL,
  key_version VARCHAR(32) NOT NULL,
  nonce VARBINARY(12) NOT NULL,
  ciphertext VARBINARY(4096) NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  CONSTRAINT fk_bp_records_owner FOREIGN KEY (owner_id) REFERENCES users(id),
  CONSTRAINT uq_bp_records_owner_request UNIQUE (owner_id, client_request_id),
  INDEX idx_bp_records_owner_measured_at (owner_id, measured_at_utc),
  INDEX idx_bp_records_owner_created_at (owner_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
