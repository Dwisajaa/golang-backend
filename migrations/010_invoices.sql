CREATE TABLE invoices (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    booking_id BIGINT UNSIGNED NOT NULL,
    invoice_number VARCHAR(255) NOT NULL,
    issued_at TIMESTAMP NOT NULL,
    due_at TIMESTAMP NULL DEFAULT NULL,
    subtotal DECIMAL(12, 2) NOT NULL DEFAULT 0.00,
    additional_cost DECIMAL(12, 2) NOT NULL DEFAULT 0.00,
    total_amount DECIMAL(12, 2) NOT NULL DEFAULT 0.00,
    status VARCHAR(255) NOT NULL DEFAULT 'unpaid',
    notes TEXT NULL,
    created_at TIMESTAMP NULL DEFAULT NULL,
    updated_at TIMESTAMP NULL DEFAULT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY invoices_booking_id_unique (booking_id),
    UNIQUE KEY invoices_invoice_number_unique (invoice_number),
    KEY invoices_status_index (status),
    CONSTRAINT fk_invoices_booking_id
        FOREIGN KEY (booking_id) REFERENCES bookings (id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;