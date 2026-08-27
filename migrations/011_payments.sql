CREATE TABLE payments (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    invoice_id BIGINT UNSIGNED NOT NULL,
    payment_code VARCHAR(255) NOT NULL,
    payment_method VARCHAR(30) NOT NULL,
    amount DECIMAL(12, 2) NOT NULL,
    paid_at TIMESTAMP NULL DEFAULT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'waiting_verification',
    proof_image VARCHAR(255) NULL DEFAULT NULL,
    customer_note TEXT NULL,
    admin_note TEXT NULL,
    verified_by BIGINT UNSIGNED NULL DEFAULT NULL,
    verified_at TIMESTAMP NULL DEFAULT NULL,
    created_at TIMESTAMP NULL DEFAULT NULL,
    updated_at TIMESTAMP NULL DEFAULT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY payments_payment_code_unique (payment_code),
    KEY payments_status_invoice_index (status, invoice_id),
    CONSTRAINT fk_payments_invoice_id
        FOREIGN KEY (invoice_id) REFERENCES invoices (id)
        ON DELETE CASCADE,
    CONSTRAINT fk_payments_verified_by
        FOREIGN KEY (verified_by) REFERENCES users (id)
        ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;