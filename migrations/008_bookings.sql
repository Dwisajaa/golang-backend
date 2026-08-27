CREATE TABLE bookings (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    booking_code VARCHAR(255) NOT NULL,
    customer_id BIGINT UNSIGNED NOT NULL,
    booking_date DATE NOT NULL,
    booking_time VARCHAR(255) NOT NULL,
    address VARCHAR(255) NOT NULL,
    address_detail VARCHAR(255) NULL DEFAULT NULL,
    latitude DECIMAL(10, 7) NULL DEFAULT NULL,
    longitude DECIMAL(10, 7) NULL DEFAULT NULL,
    customer_note TEXT NULL,
    additional_jobdesk TEXT NULL,
    subtotal DECIMAL(12, 2) NOT NULL DEFAULT 0.00,
    additional_cost DECIMAL(12, 2) NOT NULL DEFAULT 0.00,
    total_price DECIMAL(12, 2) NOT NULL DEFAULT 0.00,
    status VARCHAR(255) NOT NULL DEFAULT 'pending_payment',
    created_at TIMESTAMP NULL DEFAULT NULL,
    updated_at TIMESTAMP NULL DEFAULT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY bookings_booking_code_unique (booking_code),
    KEY bookings_status_index (status),
    KEY bookings_customer_created_index (customer_id, created_at),
    CONSTRAINT fk_bookings_customer_id
        FOREIGN KEY (customer_id) REFERENCES users (id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;