CREATE TABLE reviews (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    booking_id BIGINT UNSIGNED NOT NULL,
    customer_id BIGINT UNSIGNED NOT NULL,
    technician_id BIGINT UNSIGNED NOT NULL,
    rating TINYINT UNSIGNED NOT NULL,
    comment TEXT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'published',
    created_at TIMESTAMP NULL DEFAULT NULL,
    updated_at TIMESTAMP NULL DEFAULT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY reviews_booking_id_unique (booking_id),
    KEY reviews_status_technician_index (status, technician_id),
    CONSTRAINT fk_reviews_booking_id
        FOREIGN KEY (booking_id) REFERENCES bookings (id)
        ON DELETE CASCADE,
    CONSTRAINT fk_reviews_customer_id
        FOREIGN KEY (customer_id) REFERENCES users (id)
        ON DELETE CASCADE,
    CONSTRAINT fk_reviews_technician_id
        FOREIGN KEY (technician_id) REFERENCES users (id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;