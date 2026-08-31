# Backup and Recovery

## 1. What is backed up

1. **MySQL Database**: Contains users, profiles, catalog, bookings, invoices,
   payments, and system state.
2. **Private Storage Volume**: `/storage/app/private/payment-proofs/`
   (contains customer-uploaded bank transfer proofs).

Code/binaries are NOT backed up (rebuilt from GitHub).

## 2. When / How Often (CRON)

Daily at 02:00 server time. Keep locally for 7 days; offsite sync recommended.

```cron
0 2 * * * /opt/scripts/backup-db.sh
0 3 * * * /opt/scripts/backup-storage.sh
```

## 3. Database Backup Procedure

Use `mysqldump` with `--single-transaction` so production is not locked.

```bash
mysqldump -u apidw -p"sekret" --single-transaction --routines --triggers apidw | gzip > /backups/db-$(date +%F).sql.gz
```

## 4. Storage Backup Procedure

Tar the private volume.

```bash
tar -czf /backups/proofs-$(date +%F).tar.gz -C /opt/api-dwidev/storage/app/private/ payment-proofs
```

## 5. Restore Procedure (Disaster)

1. Stop the Go API (to prevent partial writes during restore).
   `docker compose stop api`
2. Drop and recreate the database; import SQL.
   ```bash
   mysql -u root -p -e "DROP DATABASE apidw; CREATE DATABASE apidw;"
   zcat /backups/db-YYYY-MM-DD.sql.gz | mysql -u root -p apidw
   ```
3. Extract files back to the storage mount.
   ```bash
   tar -xzf /backups/proofs-YYYY-MM-DD.tar.gz -C /opt/api-dwidev/storage/app/private/
   ```
4. Start the API.
   `docker compose start api`
5. Verify `/health`, `/ready`, and download a past payment proof.