# dbackup

A high-performance, extensible database backup CLI with built-in **deduplication**, **encryption**, **scheduling**, and **multi-cloud** storage support.

---

## Architecture

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

- **Multi-Database Support**: Native integration with PostgreSQL (Logical & Physical), MySQL/MariaDB (Logical & Physical), and SQLite (Online).
- **Content-Addressable Storage (Dedupe)**: Save massive amounts of space with parallel chunk hashing.
- **Parallel Execution**: Automatically scale your backup window with concurrent database operations and multi-threaded deduplication.
- **Multi-Cloud Storage**: Support for Local, SFTP, S3 (MinIO/AWS), FTP, and Docker.
- **Advanced Retention (GFS)**: Grandfather-Father-Son rotation (Daily, Weekly, Monthly, Yearly).
- **Storage Migration**: Move your entire backup history between storage backends with a single command.
- **Client-Side Encryption**: AES-256-GCM authenticated encryption for maximum security.

---

## Getting Started

### 1. Installation
```bash
go build -o dbackup .
sudo mv dbackup /usr/local/bin/
```

### 2. Check Environment
```bash
dbackup doctor
```

---

## Usage

### Listing Backups
Discover what's stored in your storage backend:
```bash
dbackup backups --to sftp://user@host/backups

dbackup backups --to local://./backups --db my_app_db
```

### Backing Up
```bash
dbackup backup postgres --db my_db --to s3://key:secret@localhost:9000/backups

dbackup dump --config ~/.dbackup/backup.yaml
```

### Restoring
`dbackup` makes restoration safe and intelligent:
```bash
dbackup restore mysql --name latest.manifest --to mysql://user:pass@localhost/db --confirm-restore

dbackup restore --auto --confirm-restore
```

### Migration
Move all backups from one backend to another:
```bash
dbackup migrate --from ./local-backups --to s3://bucket/archive --dedupe
```

### Advanced Retention (GFS)
```bash
dbackup backup pg --db app --keep-daily 7 --keep-weekly 4 --keep-monthly 12
```

---

## Configuration (`backup.yaml`)

`dbackup` reads from `~/.dbackup/backup.yaml` by default.

```yaml
parallelism: 4
allow_insecure: false

backups:
  - id: "prod-db"
    engine: "postgres"
    uri: "postgres://user@localhost/prod"
    to: "s3://bucket/backups?region=us-east-1"
    dedupe: true
    encrypt: true
    encryption_passphrase: "${DB_ENCRYPT_PWD}"
    retention: "30d"
    schedule: "0 2 * * *"

restores:
  - id: "weekly-verify"
    from: "s3://bucket/backups/latest.manifest"
    to: "postgres://user@localhost/verify"
    dry_run: true
```

---

## Storage Backends & URI Options

| Backend | URI Format | Key Parameters |
|---------|------------|----------------|
| **Local** | `local://./path` | |
| **SFTP** | `sftp://user:pass@host/path` | Defaults to port 22 |
| **S3 / MinIO**| `s3://ACCESS:SECRET@HOST/BUCKET` | `?region=`, `?ssl=false` |
| **FTP** | `ftp://user:pass@host/path` | Requires `--allow-insecure` |
| **Docker** | `docker://container:/path` | |

> [!TIP]
> **Performance Optimization**: dbackup uses recursive listings that automatically filter out `chunks/` to keep discovery fast even with millions of deduplicated files.

---

## Security & Reliability

- **AES-256-GCM**: Industry-standard authenticated encryption for data at rest.
- **Fast-Fail Connectivity**: S3 and SFTP operations include intelligent 15s timeouts to prevent infinite hangs.
- **Zero-Leaking**: Passwords and keys are scrubbed from logs.
- **Integrity**: Every backup is verified via SHA-256 manifest files.

---

## License
MIT License
