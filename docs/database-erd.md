# Database ERD — api-dwidev

Based on final schema from migrations audit (2026-08-27).

```mermaid
erDiagram
    users {
        bigint id PK
        varchar name
        varchar email UK
        varchar role
        timestamp email_verified_at
        varchar password
        varchar remember_token
        timestamp created_at
        timestamp updated_at
    }

    email_verification_otps {
        bigint id PK
        bigint user_id FK
        varchar type
        varchar otp_hash
        timestamp expires_at
        timestamp used_at
        tinyint attempts
        timestamp created_at
        timestamp updated_at
    }

    personal_access_tokens {
        bigint id PK
        varchar tokenable_type
        bigint tokenable_id
        varchar name
        varchar token UK
        text abilities
        timestamp last_used_at
        timestamp expires_at
        timestamp created_at
        timestamp updated_at
    }

    customer_profiles {
        bigint id PK
        bigint user_id FK_UK
        varchar full_name
        varchar phone
        varchar address
        varchar city
        varchar postal_code
        timestamp created_at
        timestamp updated_at
    }

    technician_profiles {
        bigint id PK
        bigint user_id FK_UK
        varchar technician_code UK
        varchar phone
        varchar specialization
        varchar address
        text bio
        boolean is_active
        timestamp created_at
        timestamp updated_at
    }

    service_categories {
        bigint id PK
        varchar name
        varchar slug UK
        text description
        varchar icon
        boolean is_active
        timestamp created_at
        timestamp updated_at
    }

    services {
        bigint id PK
        bigint service_category_id FK
        varchar name
        varchar slug UK
        text description
        decimal price
        varchar unit
        integer estimated_duration
        boolean is_active
        timestamp created_at
        timestamp updated_at
    }

    packages {
        bigint id PK
        varchar name
        varchar slug UK
        text description
        decimal price
        integer duration
        boolean is_active
        boolean is_popular
        timestamp created_at
        timestamp updated_at
    }

    package_items {
        bigint id PK
        bigint package_id FK
        bigint service_id FK
        integer quantity
        timestamp created_at
        timestamp updated_at
    }

    bookings {
        bigint id PK
        varchar booking_code UK
        bigint customer_id FK
        date booking_date
        varchar booking_time
        varchar address
        varchar address_detail
        decimal latitude
        decimal longitude
        text customer_note
        text additional_jobdesk
        decimal subtotal
        decimal additional_cost
        decimal total_price
        varchar status
        timestamp created_at
        timestamp updated_at
    }

    booking_items {
        bigint id PK
        bigint booking_id FK
        bigint service_id FK
        bigint package_id FK
        varchar item_type
        varchar item_name
        integer quantity
        decimal unit_price
        decimal subtotal
        timestamp created_at
        timestamp updated_at
    }

    invoices {
        bigint id PK
        bigint booking_id FK_UK
        varchar invoice_number UK
        timestamp issued_at
        timestamp due_at
        decimal subtotal
        decimal additional_cost
        decimal total_amount
        varchar status
        text notes
        timestamp created_at
        timestamp updated_at
    }

    payments {
        bigint id PK
        bigint invoice_id FK
        varchar payment_code UK
        varchar payment_method
        decimal amount
        timestamp paid_at
        varchar status
        varchar proof_image
        text customer_note
        text admin_note
        bigint verified_by FK
        timestamp verified_at
        timestamp created_at
        timestamp updated_at
    }

    booking_assignments {
        bigint id PK
        bigint booking_id FK
        bigint technician_id FK
        bigint assigned_by FK
        timestamp assigned_at
        timestamp accepted_at
        timestamp rejected_at
        timestamp started_at
        timestamp completed_at
        varchar status
        text rejection_reason
        text technician_note
        text admin_verification_note
        timestamp created_at
        timestamp updated_at
    }

    notifications {
        uuid id PK
        varchar type
        varchar notifiable_type
        bigint notifiable_id
        text data
        timestamp read_at
        timestamp created_at
        timestamp updated_at
    }

    reviews {
        bigint id PK
        bigint booking_id FK_UK
        bigint customer_id FK
        bigint technician_id FK
        tinyint rating
        text comment
        varchar status
        timestamp created_at
        timestamp updated_at
    }

    users ||--o| customer_profiles : "has one"
    users ||--o| technician_profiles : "has one"
    users ||--o{ bookings : "creates"
    users ||--o{ email_verification_otps : "has many"
    users ||--o{ booking_assignments : "assigned as technician"
    users ||--o{ reviews : "receives as technician"
    users ||--o{ notifications : "notifiable (morph)"

    service_categories ||--o{ services : "contains"
    services }o--o{ packages : "package_items pivot"
    packages ||--o{ package_items : "has items"
    services ||--o{ package_items : "referenced by"

    bookings ||--o{ booking_items : "has items"
    bookings ||--|| invoices : "has one"
    bookings ||--o{ booking_assignments : "has assignments"
    bookings ||--o| reviews : "has one"
    bookings }o--|| users : "belongs to customer"

    booking_items }o--o| services : "references (SET NULL)"
    booking_items }o--o| packages : "references (SET NULL)"

    invoices ||--o{ payments : "has payments"
    payments }o--o| users : "verified by (SET NULL)"

    booking_assignments }o--|| users : "technician"
    booking_assignments }o--o| users : "assigned by (SET NULL)"

    reviews }o--|| users : "customer"
    reviews }o--|| users : "technician"
```
