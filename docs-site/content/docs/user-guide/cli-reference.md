---
title: "CLI Reference"
weight: 1
---
# CLI Reference

The `dbackup` Command Line Interface provides comprehensive tools for managing your database backups through various unified commands.

## Global Flags

All commands support the following global flags for common configuration and overrides:

| Flag | Description | Default |
|------|-------------|---------|
| `--allow-insecure` | Allow insecure protocols (like plain FTP). | `false` |
| `--audit` | Enable tamper-evident audit logging (`audit.jsonl`). | `false` |
| `--config string` | Path to your configuration file. | `$HOME/.dbackup/backup.yaml` |
| `--confirm-restore`| Confirm destructive restore operations. | `false` |
| `-d, --db string` | Database name or file path to target. | |
| `--db-uri string` | Full database connection URI (overrides individual components). | |
| `--dedupe` | Enable storage-level deduplication (CAS). | `true` |
| `--encrypt` | Enable client-side encryption (AES-256-GCM). | `false` |
| `--encryption-key-file` | Path to the encryption key file. | |
| `--encryption-passphrase`| Passphrase for encryption key derivation. | |
| `-e, --engine string` | Database engine (`postgres`, `mysql`, `sqlite`). | |
| `--host string` | Database host. | |
| `--log-json` | Output logs in JSON format instead of plain text. | `false` |
| `--no-color` | Disable colored terminal output. | `false` |
| `--parallelism int`| Number of databases/chunks to process simultaneously. | `4` |
| `--password string`| Database password. | |
| `--port int` | Database port. | |
| `--remote-exec` | Execute backup/restore tools on the remote storage host. | `false` |
| `--slack-webhook string`| Slack Incoming Webhook URL for notifications. | |
| `--tls` | Enable TLS/SSL for database connection. | `false` |
| `--tls-ca-cert string`| Path to CA certificate for TLS verification. | |
| `--tls-client-cert` | Path to client certificate for mutual TLS (mTLS). | |
| `--tls-client-key string`| Path to client private key for mutual TLS (mTLS). | |
| `--tls-mode string`| TLS mode (`disable`, `require`, `verify-ca`, `verify-full`). | `disable` |
| `-t, --to string` | Unified targeting URI (e.g. `./local/path`, `sftp://user@host/path`).| |
| `--user string` | Database username. | |

---

## Commands

### `backup`
Runs a database backup and safely transfers it to the specified storage URI.

**Usage:** `dbackup backup [engine] [flags]`

**Available `[engine]` values:** `postgres`, `mysql`, `sqlite`

**Specific Flags:**
- `--compression-algo string`: Compression algorithm (`gzip`, `zstd`, `lz4`, `none`). Default: `lz4`.
- `--keep int`: Number of basic backups to keep.
- `--keep-daily int`: Number of daily backups to keep (GFS).
- `--keep-weekly int`: Number of weekly backups to keep (GFS).
- `--keep-monthly int`: Number of monthly backups to keep (GFS).
- `--keep-yearly int`: Number of yearly backups to keep (GFS).
- `--mysql-physical`: Use physical backup mode for MySQL instead of logical dumps. Default: `false`.
- `--name string`: Override the custom backup file/manifest name.
- `--retention string`: Retention period (e.g., `7d`, `24h`).

**Example:**
```bash
dbackup backup postgres --db my_db --to s3://my-bucket/backups --compression-algo zstd --keep-daily 7
```

### `backups`
Lists all available backups at the specified storage target.

**Usage:** `dbackup backups [flags]`

**Specific Flags:**
- `-f, --from string`: Unified source URI (alias for `--to` under this subcommand).

**Example:**
```bash
dbackup backups --to s3://my-bucket/backups --db my_db
```

### `restore`
Restores a specific backup manifest to your database.

**Usage:** `dbackup restore [engine] [flags]`

**Specific Flags:**
- `-a, --auto`: Automatically restore the latest backup (used if no explicitly named manifest is specified).
- `--dry-run`: Simulation mode; don't actually run the restore process.
- `-f, --from string`: Unified source URI for the restore target.
- `--mysql-physical`: Assume physical format instead of logical for MySQL restores.
- `--name string`: Custom backup manifest file name to restore from.

**Example:**
```bash
dbackup restore mysql --auto --to mysql://user:pass@localhost/mydb --confirm-restore
```

### `migrate`
Migrate all backup datasets and manifests intact from one storage backend to another.

**Usage:** `dbackup migrate [flags]`

**Specific Flags:**
- `--from string`: Source storage URI.
- `--to string`: Destination storage URI.
- `--dedupe`: Enable deduplication at destination. Default `true`.

**Example:**
```bash
dbackup migrate --from ./local-backups --to s3://my-bucket/backups
```

### `dump`
Reads the `backup.yaml` configuration file and executes all defined backup and restore tasks in a single go. Backups run in parallel, followed by sequential restores.

**Usage:** `dbackup dump [flags]`

**Example:**
```bash
dbackup dump --config /etc/backup.yaml
```

### `rekey`
Decrypts existing backups using an old passphrase and re-encrypts them with a new one entirely.

**Usage:** `dbackup rekey [flags]`

**Specific Flags:**
- `--target string`: Storage target URI. Default: `.`.
- `--old-pass string`: Current passphrase.
- `--new-pass string`: New passphrase.

**Example:**
```bash
dbackup rekey --target s3://my-bucket/backups --old-pass secret1 --new-pass supersecret2
```

### `doctor`
Verifies that all native tools required corresponding to each database engine (`pg_dump`, `mysqldump`, `sqlite3`, etc.) are present in your system `PATH`, and tests storage connections.

**Usage:** `dbackup doctor [flags]`

**Example:**
```bash
dbackup doctor --config backup.yaml
```
