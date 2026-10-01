# Referência de API do SDK — SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.
> Procure um símbolo com `gofi find --in sdk "<nome>"` ou abra o pacote com `gofi show`.

| Pacote | Arquivo | Resumo |
|---|---|---|
| `github.com/joaoprofile/gofi-sdk-go/base/bucket` | `base-bucket.md` | Package bucket defines a provider-agnostic object-storage abstraction. |
| `github.com/joaoprofile/gofi-sdk-go/base/bucket/buckettest` | `base-bucket-buckettest.md` | Package buckettest is the contract every bucket.Store must satisfy. |
| `github.com/joaoprofile/gofi-sdk-go/base/bucket/file` | `base-bucket-file.md` | Package file implements bucket.Store on a local directory, for development and single-node deployments. |
| `github.com/joaoprofile/gofi-sdk-go/base/bucket/mem` | `base-bucket-mem.md` | Package mem implements bucket.Store in memory, for tests and local runs. |
| `github.com/joaoprofile/gofi-sdk-go/base/bucket/oci` | `base-bucket-oci.md` | Package oci implements bucket.Store on top of OCI Object Storage. |
| `github.com/joaoprofile/gofi-sdk-go/base/bucket/s3` | `base-bucket-s3.md` | Package s3 implements bucket.Store for Amazon S3 and S3-compatible services (MinIO, Cloudflare R2, ...). |
| `github.com/joaoprofile/gofi-sdk-go/base/cloud/aws` | `base-cloud-aws.md` | Package aws loads AWS credentials for every gofi integration (S3, SQS, RDS, SigV4 signing). |
| `github.com/joaoprofile/gofi-sdk-go/base/cloud/oci` | `base-cloud-oci.md` | Package oci resolves OCI credentials for every gofi integration (object storage, queue, ...), so API keys and principals are configured once. |
| `github.com/joaoprofile/gofi-sdk-go/base/common` | `base-common.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/base/cronjob` | `base-cronjob.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/base/debug` | `base-debug.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/base/environment` | `base-environment.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/base/errs` | `base-errs.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/base/mail` | `base-mail.md` | Package mail sends transactional and bulk e-mail over any SMTP provider, with HTML + plain-text bodies, attachments and HTML templates. |
| `github.com/joaoprofile/gofi-sdk-go/base/observer` | `base-observer.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/base/secrets` | `base-secrets.md` | Package secrets resolves secret references of the form |
| `github.com/joaoprofile/gofi-sdk-go/base/secrets/awssm` | `base-secrets-awssm.md` | Package awssm resolves secret://awssm/<name-or-arn>[#key] from AWS Secrets Manager. |
| `github.com/joaoprofile/gofi-sdk-go/base/secrets/ocivault` | `base-secrets-ocivault.md` | Package ocivault resolves secret://ocivault/<secret-ocid>[#key] and secret://ocivault/<vault-ocid>/<secret-name>[#key] from OCI Vault. |
| `github.com/joaoprofile/gofi-sdk-go/base/session` | `base-session.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/base/timezone` | `base-timezone.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/base/validator` | `base-validator.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/gofi` | `gofi.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/gofi/component/cache` | `gofi-component-cache.md` | Package cache is the gofi component for the shared Redis client used by the sqln query cache and by the session component. |
| `github.com/joaoprofile/gofi-sdk-go/gofi/component/database` | `gofi-component-database.md` | Package database is the gofi component for the SQL database (sqln). |
| `github.com/joaoprofile/gofi-sdk-go/gofi/component/httpserver` | `gofi-component-httpserver.md` | Package httpserver is the gofi component for the HTTP server (netx). |
| `github.com/joaoprofile/gofi-sdk-go/gofi/component/iam` | `gofi-component-iam.md` | Package iam is the gofi component for the identity service (iam): JWT tokens, sessions with real revocation and RBAC, configured from the environment. |
| `github.com/joaoprofile/gofi-sdk-go/gofi/component/messaging` | `gofi-component-messaging.md` | Package messaging is the gofi component for the message broker (msq). |
| `github.com/joaoprofile/gofi-sdk-go/gofi/component/observability` | `gofi-component-observability.md` | Package observability is the gofi component that exports traces, metrics and logs over OTLP/gRPC. |
| `github.com/joaoprofile/gofi-sdk-go/gofi/component/session` | `gofi-component-session.md` | Package session is the gofi component that installs the global session store (base/session) backed by the driver selected by CACHE_TYPE. |
| `github.com/joaoprofile/gofi-sdk-go/gofi/config` | `gofi-config.md` | Package config is gofi's composition layer between the environment loader (base/environment) and each library's typed Config: the libraries (mail, bucket, iam, ...) stay decoupled from environment, while this package maps env vars into their explicit Config structs. |
| `github.com/joaoprofile/gofi-sdk-go/gofi/config/core` | `gofi-config-core.md` | Package core holds the process-wide settings gofi's Build applies before any component starts: timezone, logging and TLS verification. |
| `github.com/joaoprofile/gofi-sdk-go/iam` | `iam.md` | Package iam is an identity and access facade for the gofi SDK. |
| `github.com/joaoprofile/gofi-sdk-go/iam/config` | `iam-config.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/iam/core` | `iam-core.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/iam/middleware` | `iam-middleware.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/iam/port` | `iam-port.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/iam/provider/bcrypt` | `iam-provider-bcrypt.md` | Package bcrypt provides password hashing utilities for use in UserPort implementations. |
| `github.com/joaoprofile/gofi-sdk-go/iam/provider/google` | `iam-provider-google.md` | Package google implements IDPAuthPort for Google OAuth2/OIDC. |
| `github.com/joaoprofile/gofi-sdk-go/iam/provider/jwt` | `iam-provider-jwt.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/iam/provider/memory` | `iam-provider-memory.md` | Package memory implements port.SessionPort in memory. |
| `github.com/joaoprofile/gofi-sdk-go/iam/provider/microsoft` | `iam-provider-microsoft.md` | Package microsoft implements IDPAuthPort for Microsoft Identity Platform (Entra ID). |
| `github.com/joaoprofile/gofi-sdk-go/iam/provider/oidc` | `iam-provider-oidc.md` | Package oidc implements a generic IDPAuthPort for any OIDC-compliant provider. |
| `github.com/joaoprofile/gofi-sdk-go/iam/provider/password` | `iam-provider-password.md` | Package password hashes passwords with Argon2id (RFC 9106, OWASP's first choice) in the PHC string format and verifies both Argon2id and bcrypt hashes, so existing bcrypt users migrate on their next login: |
| `github.com/joaoprofile/gofi-sdk-go/iam/provider/rbac/roles` | `iam-provider-rbac-roles.md` | Package roles implements port.RBACPort with a simple role-to-permissions model. |
| `github.com/joaoprofile/gofi-sdk-go/iam/provider/redis` | `iam-provider-redis.md` | Package redis implements port.SessionPort using Redis. |
| `github.com/joaoprofile/gofi-sdk-go/iam/types` | `iam-types.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/msq` | `msq.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/msq/core` | `msq-core.md` | Package core implements the orchestration layer of the msq messaging system. |
| `github.com/joaoprofile/gofi-sdk-go/msq/msqtest` | `msq-msqtest.md` | Package msqtest is the contract every msq provider must satisfy. |
| `github.com/joaoprofile/gofi-sdk-go/msq/port` | `msq-port.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/msq/provider/kafka` | `msq-provider-kafka.md` | Package kafka implements port.Broker for Apache Kafka using the Sarama library. |
| `github.com/joaoprofile/gofi-sdk-go/msq/provider/nats` | `msq-provider-nats.md` | Package nats implements port.Broker for NATS JetStream: persistent subjects, durable consumer groups, explicit acks and redelivery. |
| `github.com/joaoprofile/gofi-sdk-go/msq/provider/oci` | `msq-provider-oci.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/msq/provider/rabbitmq` | `msq-provider-rabbitmq.md` | Package rabbitmq implements port.Broker for RabbitMQ using AMQP 0-9-1. |
| `github.com/joaoprofile/gofi-sdk-go/msq/provider/redis` | `msq-provider-redis.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/msq/provider/sqs` | `msq-provider-sqs.md` | Package sqs implements the msq broker on Amazon SQS (aws-sdk-go-v2). |
| `github.com/joaoprofile/gofi-sdk-go/msq/types` | `msq-types.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/msq/worker` | `msq-worker.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/netx` | `netx.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/netx/awssign` | `netx-awssign.md` | Package awssign signs netx requests with AWS Signature V4 (API Gateway IAM auth, OpenSearch, Lambda URLs, ...). |
| `github.com/joaoprofile/gofi-sdk-go/obs` | `obs.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/obs/logging` | `obs-logging.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/obs/metrics` | `obs-metrics.md` | Package metrics is the OpenTelemetry metric API used by gofi and by applications: it records through the global MeterProvider and links no exporter or gRPC code. |
| `github.com/joaoprofile/gofi-sdk-go/sqln` | `sqln.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/sqln/cache` | `sqln-cache.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/sqln/connection` | `sqln-connection.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/sqln/criteria` | `sqln-criteria.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/sqln/driver` | `sqln-driver.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/sqln/driver/mysql` | `sqln-driver-mysql.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/sqln/driver/oracle` | `sqln-driver-oracle.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/sqln/driver/postgres` | `sqln-driver-postgres.md` | Package postgres registers the PostgreSQL driver, built on pgx/v5. |
| `github.com/joaoprofile/gofi-sdk-go/sqln/driver/sqlserver` | `sqln-driver-sqlserver.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/sqln/filter` | `sqln-filter.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/sqln/mapping` | `sqln-mapping.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/sqln/migrate` | `sqln-migrate.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/sqln/pagination` | `sqln-pagination.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/sqln/query` | `sqln-query.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/sqln/rdsauth` | `sqln-rdsauth.md` | Package rdsauth signs RDS / Aurora IAM authentication tokens, so databases accept the pod's AWS identity instead of a static password: |
| `github.com/joaoprofile/gofi-sdk-go/sqln/statement` | `sqln-statement.md` |  |
| `github.com/joaoprofile/gofi-sdk-go/sqln/transaction` | `sqln-transaction.md` |  |
