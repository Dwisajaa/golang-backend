CREATE TABLE booking_assignments (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    booking_id BIGINT UNSIGNED NOT NULL,
    technician_id BIGINT UNSIGNED NOT NULL,
    assigned_by BIGINT UNSIGNED NULL DEFAULT NULL,
    assigned_at TIMESTAMP NULL DEFAULT NULL,
    accepted_at TIMESTAMP NULL DEFAULT NULL,
    rejected_at TIMESTAMP NULL DEFAULT NULL,
    started_at TIMESTAMP NULL DEFAULT NULL,
    completed_at TIMESTAMP NULL DEFAULT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    rejection_reason TEXT NULL,
    technician_note TEXT NULL,
    admin_verification_note TEXT NULL,
    created_at TIMESTAMP NULL DEFAULT NULL,
    updated_at TIMESTAMP NULL DEFAULT NULL,
    PRIMARY KEY (id),
    KEY booking_assignments_status_technician_index (status, technician_id),
    KEY booking_assignments_booking_id_index (booking_id),
    CONSTRAINT fk_booking_assignments_booking_id
        FOREIGN KEY (booking_id) REFERENCES bookings (id)
        ON DELETE CASCADE,
    CONSTRAINT fk_booking_assignments_technician_id
        FOREIGN KEY (technician_id) REFERENCES users (id)
        ON DELETE CASCADE,
    CONSTRAINT fk_booking_assignments_assigned_by
        FOREIGN KEY (assigned_by) REFERENCES users (id)
        ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;