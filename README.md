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
- **YAML Configuration**: Manage multiple backups and restores via a central configuration file.
- **Parallel Execution**: Automatically scale your backup window with concurrent operations.
- **Multi-Cloud Storage**: Support for Local, SFTP, FTP, Docker, and S3-compatible storage (MinIO, AWS S3).
- **Client-Side Encryption**: Secure your backups before they leave your system using AES-256-GCM.
- **Intelligent Scheduling**: Built-in cron-style and interval-based scheduling in config or CLI.
- **Simulation Mode**: Run dry-run restores to verify your backup strategy without destructive actions.
- **Environment Doctor**: Diagnoses missing native binaries (pg_dump, mysqldump, etc.) instantly.
- **Integrity Verification**: Automatic SHA-256 checksumming and manifest verification.

## Configuration

`dbackup` can read settings from `~/.dbackup/backup.yaml` or a file specified with `--config`.

### Example `backup.yaml`
```yaml
parallelism: 4
allow_insecure: false

notifications:
  slack:
    webhook_url: "https://hooks.slack.com/services/..."

backups:
  - id: "prod-db"
    engine: "postgres"
    uri: "postgres://user@localhost/prod"
    to: "s3://bucket/backups"
    encrypt: true
    encryption_passphrase: "${DB_ENCRYPT_PWD}"
    retention: "30d"
    schedule: "0 2 * * *"

  - id: "local-sqlite"
    engine: "sqlite"
    db: "./app.db"
    to: "local://./backups"
    interval: "1h"
```

## Usage

### Dump (Config-based)
Execute all tasks defined in your configuration:
```bash
dbackup dump --config my-backups.yaml
```

### Doctor (Environment Check)
Ensure your system has all required tools installed:
```bash
dbackup doctor
```

### Backup (CLI-based)
```bash
# PostgreSQL backup to S3 with encryption
dbackup backup postgres --db-uri "postgres://user@localhost/app" \
  --to s3://key:secret@minio:9000/backups \
  --encrypt --encryption-passphrase "mysecret"
```

## Storage Backends

| Backend | URI Format | Notes |
|---------|------------|-------|
| **Local** | `./path` or `local://path` | Default storage |
| **SFTP** | `user@host:/path` or `sftp://user@host/path` | Secure remote storage |
| **S3 / MinIO**| `s3://ACCESS:SECRET@HOST/BUCKET/PREFIX` | Cloud-native storage |
| **FTP** | `ftp://user:pass@host/path` | Requires --allow-insecure |
| **Docker** | `docker://container:/path` | Back up to/from containers |

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
