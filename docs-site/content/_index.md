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

{{< mermaid >}}
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
{{< /mermaid >}}

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
