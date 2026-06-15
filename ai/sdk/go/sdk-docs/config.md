# gofi — Variáveis de Ambiente (Padrão)

Este é o padrão oficial de variáveis de ambiente para todos os serviços gofi.
Use estes nomes exatos em `main.go`, `.env.example` e specs.

> **Governança:** variável fora deste padrão = desvio do SDK. Qualquer nome diferente deve ser confirmado
> com o dev antes de usar. Exceções legítimas (IDP externo, APIs de terceiros) devem ser documentadas
> explicitamente na spec. Ver regras completas em `knowledge/env-vars-standard.md`.

---

## .env completo (referência)

```env
# ── App ───────────────────────────────────────────────
APP_NAME=my-service
APP_ENVIRONMENT=development
APP_TENANT=
LOG_LEVEL=debug
APP_MAX_PARALLEL_WORKERS=10

# ── Observabilidade / OTEL ────────────────────────────
OTEL_EXPORTER_OTLP_ENDPOINT=

# ── Debug ─────────────────────────────────────────────
SERVICE_DEBUG=false
SERVICE_DEBUG_ADDR=:6060
SERVICE_DEBUG_USER=admin
SERVICE_DEBUG_PASS=

# ── Cloud ─────────────────────────────────────────────
CLOUD_PROVIDER=
CLOUD_HOST=
CLOUD_REGION=
CLOUD_SECRET=
CLOUD_TOKEN=
CLOUD_DISABLE_SSL=false

# ── Database (sqln) ───────────────────────────────────
DATABASE_DRIVER=postgres
DATABASE_MIGRATION=true
DATABASE_HOST=localhost
DATABASE_USER=postgres
DATABASE_PASSWORD=postgres
DATABASE_NAME=mydb
DATABASE_PORT=5432
DATABASE_SSL_MODE=disable
DATABASE_MAX_OPEN_CONNS=25
DATABASE_MAX_IDLE_CONNS=5
DATABASE_MAX_LIFETIME=300

# ── Cache / Redis ─────────────────────────────────────
CACHE_TYPE=redis
CACHE_URI=redis://localhost:6379
CACHE_PASSWORD=
CACHE_USE_TLS=false

# ── Mensageria (msq) ──────────────────────────────────
MESSAGING_PROVIDER=rabbitmq
MESSAGING_USER=guest
MESSAGING_PASSWORD=guest
MESSAGING_HOST=localhost
MESSAGING_PORT=5672
```

---

## Por módulo

### App (sempre obrigatório)

| Variável | Descrição | Default |
|----------|-----------|---------|
| `APP_NAME` | Nome do serviço (aparece nos logs e traces) | — |
| `APP_ENVIRONMENT` | `development`, `staging`, `production` | `development` |
| `APP_TENANT` | Tenant padrão (quando multi-tenant fixo) | — |
| `LOG_LEVEL` | `debug`, `info`, `warn`, `error` | `info` |
| `APP_MAX_PARALLEL_WORKERS` | Workers paralelos (processamento de filas/jobs) | `10` |

### Observabilidade (obs)

| Variável | Descrição |
|----------|-----------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Endereço do OTEL collector (ex: `otel-collector:4317`) — omitir para apenas stdout |

### Debug

| Variável | Descrição |
|----------|-----------|
| `SERVICE_DEBUG` | `true` para ativar pprof/debug endpoints |
| `SERVICE_DEBUG_ADDR` | Endereço do servidor de debug (ex: `:6060`) |
| `SERVICE_DEBUG_USER` | Usuário para autenticação básica no debug |
| `SERVICE_DEBUG_PASS` | Senha para autenticação básica no debug |

### Cloud

| Variável | Descrição |
|----------|-----------|
| `CLOUD_PROVIDER` | Provedor: `aws`, `gcp`, `azure`, `oci` |
| `CLOUD_HOST` | Host/endpoint do provedor |
| `CLOUD_REGION` | Região (ex: `us-east-1`) |
| `CLOUD_SECRET` | Secret key |
| `CLOUD_TOKEN` | Token/access key |
| `CLOUD_DISABLE_SSL` | `true` para desabilitar SSL (dev apenas) |

### Database (sqln)

| Variável | Descrição | Default |
|----------|-----------|---------|
| `DATABASE_DRIVER` | `postgres`, `mysql`, `sqlserver`, `oracle` | `postgres` |
| `DATABASE_MIGRATION` | `true` para rodar migrations na inicialização | `false` |
| `DATABASE_HOST` | Host do banco | `localhost` |
| `DATABASE_USER` | Usuário | — |
| `DATABASE_PASSWORD` | Senha | — |
| `DATABASE_NAME` | Nome do banco | — |
| `DATABASE_PORT` | Porta | `5432` |
| `DATABASE_SSL_MODE` | `disable`, `require`, `verify-full` | `disable` |
| `DATABASE_MAX_OPEN_CONNS` | Máximo de conexões abertas | `25` |
| `DATABASE_MAX_IDLE_CONNS` | Máximo de conexões ociosas | `5` |
| `DATABASE_MAX_LIFETIME` | Lifetime máximo de conexão em segundos | `300` |

### Cache (Redis)

| Variável | Descrição |
|----------|-----------|
| `CACHE_TYPE` | Tipo de cache: `redis` |
| `CACHE_URI` | URI de conexão (ex: `redis://localhost:6379`) |
| `CACHE_PASSWORD` | Senha do Redis |
| `CACHE_USE_TLS` | `true` para conexão TLS |

### Mensageria (msq)

| Variável | Descrição |
|----------|-----------|
| `MESSAGING_PROVIDER` | `rabbitmq`, `kafka`, `sqs`, `oci`, `redis` |
| `MESSAGING_USER` | Usuário |
| `MESSAGING_PASSWORD` | Senha |
| `MESSAGING_HOST` | Host do broker |
| `MESSAGING_PORT` | Porta (RabbitMQ: `5672`, Kafka: `9092`) |
