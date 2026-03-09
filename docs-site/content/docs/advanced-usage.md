---
title: "Advanced Usage"
weight: 40
---
# Advanced Usage

`dbackup` isn't just a simple wrap around `pg_dump`; it contains several enterprise-ready capabilities to help organize, verify, and secure your long-term storage strategy.

## Grandfather-Father-Son (GFS) Retention

If you only use `--retention 30d`, you'll store 30 daily backups. However, for compliance reasons, you often need backups covering a wider span of months and years without paying the storage cost of 365+ daily backups. `dbackup` completely natively implements GFS rules:

```bash
dbackup backup pg --db app \
    --keep-daily 7 \
    --keep-weekly 4 \
    --keep-monthly 12
```

This ensures you have fine-grained recovery points up to a week old, 4 weekly snapshots for the last month, and 1 snapshot for each month of the past year.

## Client-Side Encryption & Key Rotation

Providing `--encrypt` seamlessly seals your snapshots using Authenticated AES-256-GCM. But what happens if you have employee turnover and need to rotate your passwords? 

Instead of re-running full backups, use `dbackup rekey`:
```bash
dbackup rekey --target s3://bucket/backups --old-pass secret1 --new-pass secret2
```
This carefully updates internal chunking indexes locally, avoiding enormous IO streams while guaranteeing nobody with old passphrases can decrypt the manifest definitions for snapshots.

## Tamper-Evident Audit Logging

For security architectures requiring proof of access, adding the `--audit` flag creates a cryptographically chained `audit.jsonl` log alongside your normal storage. Future append activities calculate a SHA hash of the previous state + the new event, proving no past logs have been silently mutated.

```bash
dbackup dump --audit
```

## Content-Addressable Storage (Deduplication)

When `--dedupe` is enabled (which is the default behavior), backups aren't stored as a single massive gzip. They are split into cryptographic blocks (chunks). A single byte change in the database only results in that new chunk being uploaded, meaning keeping 365 daily backups usually costs nearly the same as keeping ~7 non-deduped backups.
