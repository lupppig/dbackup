---
title: "Quick Start"
weight: 2
---
# Quick Start

Once you have installed `dbackup`, you can easily start backing up and restoring your databases.

## 1. Check Environment

Before running backups, it's a good idea to verify that `dbackup` recognizes the native database tools (like `pg_dump`, `mysqldump`, `sqlite3`).

```bash
# Check local binaries
dbackup doctor

# Check live target connectivity & permissions (if you have a config file)
dbackup doctor --config backup.yaml
```

## 2. Your First Backup

To back up a local database directly to a local directory:

```bash
dbackup backup postgres --db my_db --to ./local-backups
```
This command streams the backup into chunked, deduplicated storage in `./local-backups`.

## 3. Listing Backups

To see what's stored in your storage backend:

```bash
dbackup backups --to ./local-backups --db my_db
```

## 4. Restoring a Backup

If you want to restore the latest backup intelligently:

```bash
dbackup restore postgres --auto --to postgres://user:pass@localhost/my_db_restored --confirm-restore
```

## 5. Using Configuration Files

For daily usage, it is recommended to use an declarative configuration file. Read more in the [Configuration](../user-guide/configuration.md) section.

```bash
# Execute all backups/restores defined in backup.yaml
dbackup dump --config ~/.dbackup/backup.yaml
```
