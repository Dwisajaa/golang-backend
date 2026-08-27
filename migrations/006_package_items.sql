CREATE TABLE package_items (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    package_id BIGINT UNSIGNED NOT NULL,
    service_id BIGINT UNSIGNED NOT NULL,
    quantity INT NOT NULL DEFAULT 1,
    created_at TIMESTAMP NULL DEFAULT NULL,
    updated_at TIMESTAMP NULL DEFAULT NULL,
    PRIMARY KEY (id),
    KEY package_items_package_service_index (package_id, service_id),
    CONSTRAINT fk_package_items_package_id
        FOREIGN KEY (package_id) REFERENCES packages (id)
        ON DELETE CASCADE,
    CONSTRAINT fk_package_items_service_id
        FOREIGN KEY (service_id) REFERENCES services (id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;