# dbackup

A high-performance, extensible database backup CLI with built-in encryption, scheduling, and multi-cloud storage support.

## Overview

dbackup is a modern CLI tool designed to simplify and automate database backup workflows. It focuses on reliability, security, and developer productivity by providing a unified interface for various databases and storage targets.

### Architecture

```text
    +-----------+       +------------------------+       +-------------------+
    |  dbackup  | ----> | Backup/Restore Manager | ----> | Database Adapters |
    |    CLI    |       +-----------+------------+       +---------+---------+
    +-----------+                   |                              |
                                    v                              v
                        +-----------+------------+       +---------+----------+
                        | Storage & Crypto Logic |       | Postgres, MySQL,   |
                        +-----------+------------+       | SQLite             |
                                    |                    +--------------------+
                                    v
                        +-----------+------------+
                        | Local, SFTP, FTP, S3,  |
                        | Docker Volumes         |
                        +------------------------+
```

## Features

- **Multi-Database Support**: Native integration with PostgreSQL, MySQL/MariaDB, and SQLite.
- **Multi-Cloud Storage**: Support for Local, SFTP, FTP, Docker, and S3-compatible storage (MinIO, AWS S3).
- **Client-Side Encryption**: Secure your backups before they leave your system using AES-256-GCM.
- **Parallel Execution**: Scale your backup window with concurrent operations via --parallelism.
- **Intelligent Scheduling**: Built-in cron-style and interval-based scheduling.
- **Simulation Mode**: Run dry-run restores to verify your backup strategy without destructive actions.
- **Environment Doctor**: Diagnoses missing native binaries (pg_dump, mysqldump, etc.) instantly.
- **Integrity Verification**: Automatic SHA-256 checksumming and manifest verification.

## Storage Backends

| Backend | URI Format | Notes |
|---------|------------|-------|
| **Local** | `./path` or `local://path` | Default storage |
| **SFTP** | `user@host:/path` or `sftp://user@host/path` | Secure remote storage |
| **S3 / MinIO**| `s3://ACCESS:SECRET@HOST/BUCKET/PREFIX` | Cloud-native storage |
| **FTP** | `ftp://user:pass@host/path` | Requires --allow-insecure |
| **Docker** | `docker://container:/path` | Back up to/from containers |

## Usage

### Doctor (Environment Check)
Ensure your system has all required tools installed:
```bash
dbackup doctor
```

### Backup
```bash
# PostgreSQL backup to S3 with encryption
dbackup backup postgres --db-uri "postgres://user@localhost/app" \
  --to s3://key:secret@minio:9000/backups \
  --encrypt --encryption-passphrase "mysecret"

# Parallel SQLite backups
dbackup backup sqlite --db db1.sqlite --db db2.sqlite --to ./backups --parallelism 2
```

### Restore (Dry-Run)
Test your restore without touching the database:
```bash
dbackup restore postgres --from ./backups/prod.manifest --dry-run
```

## Scheduling

Schedule recurring tasks easily:
```bash
# Every morning at 3 AM
dbackup schedule backup mysql --db mydb --to sftp://user@remote:/backups --cron "0 3 * * *"

# Every hour
dbackup schedule backup sqlite --db app.db --to ./backups --interval 1h
```

## Security

- **AES-256-GCM**: Industry-standard authenticated encryption for data at rest.
- **PBKDF2**: Secure key derivation from passphrases.
- **Zero-Leaking Logs**: Passwords and keys are automatically scrubbed from logs and terminals.

## Development

```bash
go build -o dbackup .
go test ./...
```

## License

MIT License
