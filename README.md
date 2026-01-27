# dbackup

`dbackup` is a high-performance, extensible database backup engine designed for security-conscious environments. It provides a unified interface for managing backups across disparate database architectures and storage providers.

Currently in active development, `dbackup` prioritizes reliability, technical excellence, and seamless integration with modern infrastructure.

## Key Capabilities

- **Unified Interface**: Decoupled architecture using a pluggable adapter system (`DBAdapter`).
- **Enterprise-Grade Security**: First-class support for SSL/TLS, including mutual TLS (mTLS) for high-security database environments.
- **Flexible Connectivity**: Support for standard connection parameters and full DSU (Data Source Uniform) URIs.
- **Stateless Operations**: Designed for containerized environments and scheduled orchestration.

## Current Support Matrix

| Feature | Support Status | Target Engine/Driver |
| :--- | :--- | :--- |
| **Databases** | Active | PostgreSQL (`lib/pq`) |
| **Security** | Supported | TLS/SSL, mTLS, Certificate Verification |
| **Output** | Supported | Local File System |
| **Roadmap** | Planned | MySQL, MongoDB, S3/Remote Storage |

## Usage Orchestration

### Authentication and Connection
`dbackup` supports both granular flags and connection URIs. 

#### PostgreSQL via Flags
```bash
./bin/dbackup backup \
  --db postgres \
  --host db.example.com \
  --user admin \
  --password secret \
  --dbname production
```

#### Secure mTLS Connection
```bash
./bin/dbackup backup \
  --db postgres \
  --db-uri "postgres://admin:secret@db.example.com:5432/prod" \
  --tls \
  --tls-mode verify-full \
  --tls-ca-cert /etc/ssl/certs/ca.pem \
  --tls-client-cert /etc/ssl/client.crt \
  --tls-client-key /etc/ssl/client.key
```

## Engineering and Development

### Testing Philosophy
The project maintains a rigorous testing suite using `testify` for contract validation and connection string integrity.

```bash
make test
```

### Build Requirements
- Go 1.22 or higher
- `make` for build automation

## Technical Overview
The project is organized to prioritize separation of concerns:
- `/cmd`: CLI entry point and flag orchestration (SPF13/Cobra).
- `/internal/db`: Core database adapter interfaces and engine-specific implementations.
- `/internal/storage`: Abstraction layer for backup persistence.

## License
MIT License - Developed and maintained by the `dbackup` contributors.
