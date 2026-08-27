CREATE TABLE booking_items (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    booking_id BIGINT UNSIGNED NOT NULL,
    service_id BIGINT UNSIGNED NULL DEFAULT NULL,
    package_id BIGINT UNSIGNED NULL DEFAULT NULL,
    item_type VARCHAR(20) NOT NULL,
    item_name VARCHAR(255) NOT NULL,
    quantity INT NOT NULL DEFAULT 1,
    unit_price DECIMAL(12, 2) NOT NULL DEFAULT 0.00,
    subtotal DECIMAL(12, 2) NOT NULL DEFAULT 0.00,
    created_at TIMESTAMP NULL DEFAULT NULL,
    updated_at TIMESTAMP NULL DEFAULT NULL,
    PRIMARY KEY (id),
    KEY booking_items_booking_type_index (booking_id, item_type),
    CONSTRAINT fk_booking_items_booking_id
        FOREIGN KEY (booking_id) REFERENCES bookings (id)
        ON DELETE CASCADE,
    CONSTRAINT fk_booking_items_service_id
        FOREIGN KEY (service_id) REFERENCES services (id)
        ON DELETE SET NULL,
    CONSTRAINT fk_booking_items_package_id
        FOREIGN KEY (package_id) REFERENCES packages (id)
        ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;