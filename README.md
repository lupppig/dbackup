# dbackup

A high-performance, extensible database backup CLI with built-in encryption, scheduling, and remote storage support.

## Features

- **Multi-Database Support**: PostgreSQL, MySQL, SQLite, Redis
- **Client-Side Encryption**: AES-256-GCM with PBKDF2 key derivation
- **Parallel Execution**: Back up multiple databases concurrently (`--parallelism`)
- **Storage Backends**: Local, SFTP (remote VMs), FTP, Docker volumes
- **Unified Scheduler**: Cron-style and interval-based recurring backups
- **Slack Notifications**: Task success/failure alerts via webhook
- **Integrity Verification**: SHA-256 manifest checksums

## Quick Start

### Backup
```bash
# SQLite backup to local directory
dbackup backup sqlite --db myapp.db --to ./backups

# PostgreSQL backup to remote VM with encryption
dbackup backup postgres --db-uri "postgres://user:pass@localhost/mydb" \
  --to user@backup-server:/var/backups \
  --encrypt --encryption-passphrase "secret"

# Multiple databases in parallel
dbackup backup postgres://db1 postgres://db2 postgres://db3 --to ./backups --parallelism 3
```

### Restore
```bash
dbackup restore sqlite --from ./backups/myapp.manifest --db restored.db --confirm-restore
```

### Scheduling
```bash
# Schedule daily backup at 2 AM (runs in background)
dbackup schedule backup sqlite --db mydb.db --to user@server:/backups --cron "0 2 * * *" --encrypt

# Schedule hourly backup
dbackup schedule backup postgres --db-uri "postgres://..." --to ./backups --interval 1h

# List scheduled tasks
dbackup schedule list

# Remove a scheduled task
dbackup schedule remove <TASK_ID>
```

## Security

| Feature | Implementation |
|---------|---------------|
| Encryption | AES-256-GCM |
| Key Derivation | PBKDF2 |
| Integrity | SHA-256 manifests |
| Secret Handling | Never logged, env var support (`DBACKUP_KEY`) |
| Restore Safety | Requires `--confirm-restore` |

## Storage Backends

| Backend | URI Format |
|---------|------------|
| Local | `./path` or `local://path` |
| SFTP | `user@host:/path` or `sftp://user@host/path` |
| FTP | `ftp://user:pass@host/path` (requires `--allow-insecure`) |
| Docker | `docker://container:/path` |

## Flags

| Flag | Description |
|------|-------------|
| `--parallelism` | Number of concurrent backup/restore operations |
| `--encrypt` | Enable AES-256-GCM encryption |
| `--encryption-key-file` | Path to 32-byte key file |
| `--encryption-passphrase` | Passphrase for key derivation |
| `--cron` | Cron expression for scheduling |
| `--interval` | Interval for scheduling (e.g., `1h`, `30m`) |
| `--slack-webhook` | Slack webhook URL for notifications |
| `--confirm-restore` | Required for restore operations |
| `--verify` | Verify backup integrity after completion |

## Development

```bash
# Build
go build -o dbackup .

# Test
go test ./...

# Run all tests with verbose output
go test -v ./...
```

## License

MIT License
