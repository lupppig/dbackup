---
title: "Configuration"
weight: 2
---
# Configuration File

While `dbackup` extensively supports command line options, defining your tasks in a centralized `backup.yaml` makes automation and CI/CD much easier.

By default, `dbackup` checks `$HOME/.dbackup/backup.yaml`. You can override this using the global `--config` flag.

## Schema Overview

```yaml
parallelism: 4
allow_insecure: false

backups:
  - id: "prod-db"
    engine: "postgres" # postgres, mysql, sqlite
    uri: "postgres://user@localhost/prod"
    to: "s3://bucket/backups?region=us-east-1"
    dedupe: true
    encrypt: true
    encryption_passphrase: "${DB_ENCRYPT_PWD}" # Can use env vars
    retention: "30d"
    schedule: "0 2 * * *" # Optional Cron formatting for internal scheduler

    # Advanced GFS settings
    keep: 0
    keep_daily: 7
    keep_weekly: 4
    keep_monthly: 12
    keep_yearly: 1

restores:
  - id: "weekly-verify"
    from: "s3://bucket/backups/latest.manifest"
    to: "postgres://user@localhost/verify"
    dry_run: true
    auto: true # Grabs latest

notifications:
  slack:
    webhook_url: "${SLACK_URL}"
    template: "🚀 {{.Database}} backup finished in {{.FormattedDuration}}"
  webhooks:
    - id: "discord"
      url: "https://discord.com/api/webhooks/..."
      template: '{"content": "Backup of {{.Database}} [{{.Status}}]"}'
```

## Storage Backends & URI Options

`dbackup` employs a unified URI targeting standard. Instead of writing separate configurations for each cloud layout, you encode details in the URI.

| Backend | URI Format | Description | Attributes / Keys |
|---------|------------|-------------|-------------------|
| **Local** | `local://./path` or `/absolute/path` | Native disk backups. | None |
| **SFTP** | `sftp://user:pass@host/path` | Secure FTP transfer. | Defaults to port 22 |
| **S3 / MinIO**| `s3://ACCESS:SECRET@HOST/BUCKET` | Standard Object storage. | `?region=eu-central-1`, `?ssl=false` |
| **FTP** | `ftp://user:pass@host/path` | Standard FTP server. | Requires `--allow-insecure` to function |
| **Docker** | `docker://container:/path` | Push directly inside containers. | |

## Environment Variables

For security, password fields in the YAML like `encryption_passphrase` and connection strings (e.g. `uri`, `to`) will automatically interpolate environment variables when defined with `${VAR_NAME}` syntax.
