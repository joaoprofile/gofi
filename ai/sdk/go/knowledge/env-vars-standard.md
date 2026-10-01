---
name: env-vars-standard
description: Governança de variáveis de ambiente — nomes padrão lidos pelo gofi (base/environment), X_FILE, secret://, desvios comuns e protocolo para variável nova
sdk: v0.8.2
keywords: [env, variáveis de ambiente, APP_NAME, APP_ENVIRONMENT, DATABASE_, CACHE_, MESSAGING_, BUCKET_, OTEL_, JWT_, TIMEZONE, secret://, X_FILE, padrão]
---

# Governança de Variáveis de Ambiente

## Regra fundamental

**Toda variável lida por um serviço gofi usa o nome que o SDK já define**
(struct `environment.Environment` — `.claude/sdk/go/api/base-environment.md`).
Variável fora do padrão só entra confirmada pelo dev e documentada na spec
(§ Variáveis de Ambiente). Como o valor chega ao código (carga, `.env`,
segredos, `Config` tipado): `configuration.md`.

Todo nome abaixo aceita:
- **`X_FILE`** — caminho de arquivo com o valor (segredo montado em volume);
- **`secret://<provider>/<nome>[#chave]`** — resolvido no boot (`env`, `file`
  embutidos; `awssm`, `ocivault` por import/registro).

---

## Variáveis padrão por módulo

| Módulo | Variáveis |
|--------|-----------|
| App | `APP_NAME` (sobrescrito pelo nome em `gofi.New`), `APP_VERSION`, `APP_ENVIRONMENT` (`dev`\|`stage`\|`test`\|`prod`), `APP_TENANT`, `APP_MAX_PARALLEL_WORKERS` (default 1), `PORT` |
| Carga | `GOFI_DOTENV` (`true`\|`false` força leitura do `.env`; default: lê fora de `prod`/`stage`) |
| Timezone | `TIMEZONE` (IANA; vazio = UTC) |
| Logging | `LOG_LEVEL` (`debug`\|`info`\|`warn`\|`error`; default info) |
| Observabilidade | `OTEL_EXPORTER_OTLP_ENDPOINT` (vazio = telemetria desligada), `OTEL_EXPORTER_OTLP_HEADERS`, `OTEL_EXPORTER_OTLP_INSECURE` (`false` liga TLS), `OTEL_RESOURCE_ATTRIBUTES`, `OTEL_METRIC_EXPORT_INTERVAL`, `GOFI_OTEL_LEGACY_METRIC_NAMES` |
| Debug (pprof, fora de prod) | `SERVICE_DEBUG`, `SERVICE_DEBUG_ADDR`, `SERVICE_DEBUG_USER`, `SERVICE_DEBUG_PASS` |
| HTTP | `ALLOWED_ORIGINS` (CSV), `TLS_INSECURE_SKIP_VERIFY` (nunca em prod) |
| Database | `DATABASE_DRIVER` (default `postgres`), `DATABASE_HOST`, `DATABASE_PORT`, `DATABASE_NAME`, `DATABASE_USER`, `DATABASE_PASSWORD`, `DATABASE_SSL_MODE`, `DATABASE_MIGRATION`, `DATABASE_MAX_OPEN_CONNS`, `DATABASE_MAX_IDLE_CONNS`, `DATABASE_MAX_LIFETIME`, `DATABASE_MAX_IDLE_TIME`, `DATABASE_READ_HOST`, `DATABASE_READ_PORT` |
| Cache / sessão | `CACHE_TYPE` (`redis`\|`oci`), `CACHE_URI`, `CACHE_PASSWORD`, `CACHE_USE_TLS` |
| Mensageria | `MESSAGING_PROVIDER` (`kafka`\|`rabbitmq`\|`sqs`\|`oci`\|`redis`\|`nats`), `MESSAGING_HOST`, `MESSAGING_PORT`, `MESSAGING_USER`, `MESSAGING_PASSWORD`, `MESSAGING_USE_TLS`, `MESSAGING_SASL_MECHANISM`, `MESSAGING_POLLING_INTERVAL`, `MESSAGING_ENCODING` (`envelope`\|`cloudevents`), `MESSAGING_REDIS_MODE` (`pubsub`\|`streams`), `MESSAGING_OCI_AUTH_MODE`, `MESSAGING_OCI_TENANCY_ID`, `MESSAGING_OCI_USER_ID`, `MESSAGING_OCI_REGION`, `MESSAGING_OCI_FINGERPRINT`, `MESSAGING_OCI_PRIVATE_KEY`, `MESSAGING_DEBUG` (Kafka) |
| Bucket | `BUCKET_PROVIDER`, `BUCKET_NAME`, `BUCKET_REGION`, `BUCKET_ENDPOINT`, `BUCKET_S3_ACCESS_KEY`, `BUCKET_S3_SECRET_KEY`, `BUCKET_S3_USE_SSL`, `BUCKET_OCI_AUTH_MODE`, `BUCKET_OCI_NAMESPACE`, `BUCKET_OCI_TENANCY_ID`, `BUCKET_OCI_USER_ID`, `BUCKET_OCI_FINGERPRINT`, `BUCKET_OCI_PRIVATE_KEY`, `BUCKET_OCI_PASSPHRASE` |
| Cloud | AWS: cadeia padrão (`AWS_REGION`, `AWS_PROFILE`, IRSA / EKS Pod Identity). OCI: `OCI_PRIVATE_KEY` (chave API compartilhada), modo por recurso (`*_OCI_AUTH_MODE`) |
| Auth / IAM | `JWT_SECRET` (32+ bytes), `JWT_ISSUER` (default `APP_NAME`), `JWT_KEY_ID`, `JWT_PREVIOUS_KEY_ID`, `JWT_PREVIOUS_SECRET`, `ACCESS_TOKEN_TTL` (default 15m), `REFRESH_TOKEN_TTL` (default 168h) |
| OAuth | `OAUTH_GOOGLE_CLIENT_ID`, `OAUTH_GOOGLE_CLIENT_SECRET`, `OAUTH_GOOGLE_REDIRECT_URI` |
| Mail (SMTP) | `MAIL_HOST`, `MAIL_PORT`, `MAIL_USERNAME`, `MAIL_PASSWORD`, `MAIL_FROM_NAME`, `MAIL_FROM_EMAIL`, `MAIL_ENCRYPTION` (`none`\|`starttls`\|`tls`), `MAIL_AUTH` (`plain`\|`login`\|`cram-md5`\|`none`), `MAIL_TIMEOUT`, `MAIL_MAX_RETRIES`, `MAIL_POOL_SIZE`, `MAIL_HELO_DOMAIN` |

Não existem mais no SDK: `CLOUD_PROVIDER`, `CLOUD_HOST`, `CLOUD_REGION`,
`CLOUD_SECRET`, `CLOUD_TOKEN`, `CLOUD_DISABLE_SSL` — identidade de nuvem vem da
cadeia padrão AWS ou do modo OCI de cada recurso.

---

## O que NÃO é padrão (desvios comuns)

| Variável fora do padrão | Padrão correto |
|-------------------------|----------------|
| `DATABASE_URL`, `DB_HOST`, `DB_USER` | `DATABASE_HOST` + `DATABASE_PORT` + `DATABASE_USER` … |
| `REDIS_ADDR`, `REDIS_URL` | `CACHE_URI` |
| `REDIS_PASSWORD` | `CACHE_PASSWORD` |
| `RABBIT_HOST`, `KAFKA_BROKERS` | `MESSAGING_HOST` (+ `MESSAGING_PORT`) |
| `MQ_USER` | `MESSAGING_USER` |
| `GOOGLE_CLIENT_ID` | `OAUTH_GOOGLE_CLIENT_ID` |
| `CORS_ORIGINS`, `CORS_*` | `ALLOWED_ORIGINS` |
| `TZ` como fuso do serviço | `TIMEZONE` |
| `SMTP_HOST`, `SMTP_*` | `MAIL_HOST`, `MAIL_*` |
| `DATABASE_PASSWORD=<texto>` em manifesto | `DATABASE_PASSWORD=secret://…` ou `DATABASE_PASSWORD_FILE` |

---

## Quando uma variável nova é legítima

Variável fora da tabela existe quando pertence ao **próprio serviço** ou a
integração que o SDK não gerencia:

- Parâmetro de negócio/operação do serviço: horário de cron, concorrência de
  consumer, feature flag, timeout de integração (`{CONTEXTO}_CRON_HOUR`,
  `{CONTEXTO}_CONSUMER_CONCURRENCY`, `{CONTEXTO}_ENABLED`).
- Credencial de API de terceiro sem provider no SDK: `{FORNECEDOR}_API_KEY` —
  valor sempre `secret://` ou `_FILE`.

Convenção: `UPPER_SNAKE_CASE`, prefixo = contexto ou integração; unidade no
tipo (`time.Duration` → `30s`, `5m`), não no nome. Lida **só** em `config.go`
via struct com tag `env` (`configuration.md` § `Config` do projeto).

---

## Protocolo de confirmação

Quando qualquer agent identificar uma variável não catalogada:

1. **NÃO use a variável fora do padrão silenciosamente.**
2. Pergunte ao dev:
   > "A variável `{VAR_NAME}` não faz parte do padrão gofi. Confirma que devemos usá-la? Se sim, para qual propósito?"
3. Com a confirmação, documente na spec (§ Variáveis de Ambiente) com default
   e se é segredo.
