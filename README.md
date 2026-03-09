# dbackup

[![roadmap.sh](https://img.shields.io/badge/roadmap.sh-Database_Backup_Utility-blue)](https://roadmap.sh/projects/database-backup-utility)

A high-performance, extensible database backup CLI with built-in **deduplication**, **encryption**, **scheduling**, and **multi-cloud** storage support.

> Based on the [Database Backup Utility](https://roadmap.sh/projects/database-backup-utility) project from roadmap.sh.

---

## Architecture

```mermaid
flowchart TB
    %% Core CLI
    CLI(fa:fa-terminal dbackup CLI)

    %% Managers
    subgraph Core[Backup/Restore Manager]
        direction TB
        Sched(Scheduling & Setup)
        Val(Integrity Validation)
        Audit(Tamper-Evident Audit Log)
    end
    
    %% Engine Connectors
    subgraph Adapters[Database Adapters]
        direction LR
        PG[(PostgreSQL)]
        My[(MySQL/MariaDB)]
        SQ[(SQLite)]
    end

    %% Pipeline Logic
    subgraph Pipe[Processing Pipeline]
        direction TB
        CAS(fa:fa-compress Chunking & Deduplication)
        Crypto(fa:fa-lock AES-256-GCM Encryption)
        Manifest(SHA-256 Manifest Generation)
        CAS --> Crypto --> Manifest
    end

    %% Storage Backends
    subgraph Storage[Multi-Cloud Storage Backends]
        direction LR
        S3(fa:fa-cloud S3 / MinIO)
        SFTP(fa:fa-server SFTP / FTP)
        Local(fa:fa-folder Local Disk)
        Docker(fa:fa-docker Docker Volumes)
    end

    %% Connections
    CLI ==>|Command / Config| Core
    Core -->|1. Extract Stream| Adapters
    Adapters -->|2. Raw Data Stream| Pipe
    Pipe ==>|3. Encrypted Blocks & Manifests| Storage

    %% Styling
    classDef cli fill:#1f2937,color:#fff,stroke:#4b5563,stroke-width:2px,rx:8px,ry:8px;
    classDef core fill:#2563eb,color:#fff,stroke:#1d4ed8,stroke-width:2px;
    classDef adapter fill:#059669,color:#fff,stroke:#047857,stroke-width:2px;
    classDef pipe fill:#7c3aed,color:#fff,stroke:#6d28d9,stroke-width:2px;
    classDef storage fill:#db2777,color:#fff,stroke:#be185d,stroke-width:2px;
    
    class CLI cli;
    class Sched,Val,Audit core;
    class PG,My,SQ adapter;
    class CAS,Crypto,Manifest pipe;
    class S3,SFTP,Local,Docker storage;
```

## Features

- **Multi-Database Support**: Native integration with PostgreSQL (Logical & Physical), MySQL/MariaDB (Logical & Physical), and SQLite (Online).
- **Content-Addressable Storage (Dedupe)**: Save massive amounts of space with parallel chunk hashing.
- **Parallel Execution**: Automatically scale your backup window with concurrent database operations and multi-threaded deduplication.
- **Multi-Cloud Storage**: Support for Local, SFTP, S3 (MinIO/AWS), FTP, and Docker.
- **Advanced Retention (GFS)**: Grandfather-Father-Son rotation (Daily, Weekly, Monthly, Yearly).
- **Storage Migration**: Move your entire backup history between storage backends with a single command.
- **Client-Side Encryption**: AES-256-GCM authenticated encryption for maximum security.
- **Tamper-Evident Audit Log**: Optional cryptographic chaining for all storage operations.
- **Key Rotation**: Securely re-encrypt your entire history with a new passphrase.
- **Live Diagnostics**: Built-in latency and permission checks for all configured targets.

---

## Getting Started

### 1. Installation

**Linux & macOS:**
Using Go:
```bash
go install github.com/lupppig/dbackup@latest

```
Or using Make:
```bash
make build
# The executable will be in the bin/ directory
```

**Windows (PowerShell):**
Using Go:
```powershell
go install github.com/lupppig/dbackup@latest
```
Or using Make (Requires a Make port like `mingw32-make` or WSL):
```powershell
make build
# The executable will be in the bin/ directory
```

### 2. Running Locally

**Linux & macOS:**
```bash
make run
```
Or run using Go:
```bash
go run main.go --help
```

**Windows (PowerShell):**
```powershell
make run # If Make is installed
```
Or run using Go:
```powershell
go run main.go --help
```

### 3. Check Environment
```bash
# Check local binaries
dbackup doctor

# Check live target connectivity & permissions
dbackup doctor --config backup.yaml
```
---

## Usage

### Interactive Dashboard & Documentation
`dbackup` includes an embedded web server that hosts both a graphical React dashboard and the official Hugo documentation site.

```bash
# Starts the server on port 8080
dbackup ui

# Or specify a custom port
dbackup ui --port 9000
```
- **Dashboard**: `http://localhost:8080/`
- **Documentation**: `http://localhost:8080/docs/`

> Note: To build the binary with the UI included from source, you must compile it using `make build`.

### Listing Backups
Discover what's stored in your storage backend:
```bash
dbackup backups --to user@host/backups

dbackup backups --to ./backups --db my_app_db
```

### Backing Up
```bash
dbackup backup postgres --db my_db --to key:secret@localhost:9000/backups

dbackup dump --config ~/.dbackup/backup.yaml
```

### Restoring
`dbackup` makes restoration safe and intelligent:
```bash
dbackup restore mysql --name latest.manifest --to user:pass@localhost/db --confirm-restore

dbackup restore --auto --confirm-restore
```

### Migration
Move all backups from one backend to another:
```bash
dbackup migrate --from ./local-backups --to bucket/archive --dedupe
```

### Advanced Retention (GFS)
```bash
dbackup backup pg --db app --keep-daily 7 --keep-weekly 4 --keep-monthly 12
```

### Key Rotation
Rotate your encryption secrets without losing history:
```bash
dbackup rekey --old-pass secret1 --new-pass secret2 --target bucket
```

### Audit Logging
Enable tamper-evident logging for any command:
```bash
dbackup dump --audit
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

notifications:
  slack:
    webhook_url: "${SLACK_URL}"
    template: "🚀 {{.Database}} backup finished in {{.FormattedDuration}}"
  webhooks:
    - id: "discord"
      url: "https://discord.com/api/webhooks/..."
      template: '{"content": "Backup of {{.Database}} [{{.Status}}]"}'
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
- **Tamper-Evidence**: Audit logs are hash-chained (`audit.jsonl`), preventing undetected log manipulation.
- **Key Rotation**: Dedicated `rekey` command allowing periodic security updates for long-term archives.
- **Fast-Fail Connectivity**: S3 and SFTP operations include intelligent 15s timeouts to prevent infinite hangs.
- **Zero-Leaking**: Passwords and keys are scrubbed from logs and configuration dumps.
- **Integrity**: Every backup is verified via SHA-256 manifest files.

---

## License
MIT License
