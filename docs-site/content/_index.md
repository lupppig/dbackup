---
title: "Introduction"
type: docs
weight: 1
---

# dbackup

[![roadmap.sh](https://img.shields.io/badge/roadmap.sh-Database_Backup_Utility-blue)](https://roadmap.sh/projects/database-backup-utility)

A high-performance, extensible database backup CLI with built-in **deduplication**, **encryption**, **scheduling**, and **multi-cloud** storage support.

> Based on the [Database Backup Utility](https://roadmap.sh/projects/database-backup-utility) project from roadmap.sh.

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
- **Tamper-Evident Audit Log**: Optional cryptographic chaining for all storage operations.
- **Key Rotation**: Securely re-encrypt your entire history with a new passphrase.
- **Live Diagnostics**: Built-in latency and permission checks for all configured targets.
