# Governança de Variáveis de Ambiente

## Regra fundamental

**Toda variável de ambiente usada em um serviço gofi deve seguir o padrão documentado em `gofi-sdk/config.md`.**

Variáveis fora do padrão devem ser confirmadas pelo dev antes de serem usadas ou documentadas na spec.

---

## Variáveis padrão por módulo

| Módulo | Prefixo(s) obrigatórios |
|--------|------------------------|
| App | `APP_NAME`, `APP_ENVIRONMENT`, `APP_TENANT`, `LOG_LEVEL`, `APP_MAX_PARALLEL_WORKERS` |
| Obs/OTEL | `OTEL_EXPORTER_OTLP_ENDPOINT` |
| Debug | `SERVICE_DEBUG`, `SERVICE_DEBUG_ADDR`, `SERVICE_DEBUG_USER`, `SERVICE_DEBUG_PASS` |
| Cloud | `CLOUD_PROVIDER`, `CLOUD_HOST`, `CLOUD_REGION`, `CLOUD_SECRET`, `CLOUD_TOKEN`, `CLOUD_DISABLE_SSL` |
| Database | `DATABASE_DRIVER/HOST/USER/PASSWORD/NAME/PORT/SSL_MODE/MAX_OPEN_CONNS/MAX_IDLE_CONNS/MAX_LIFETIME` |
| Cache | `CACHE_TYPE`, `CACHE_URI`, `CACHE_PASSWORD`, `CACHE_USE_TLS` |
| Mensageria | `MESSAGING_PROVIDER`, `MESSAGING_USER`, `MESSAGING_PASSWORD`, `MESSAGING_HOST`, `MESSAGING_PORT` |

---

## O que NÃO é padrão (exemplos de desvios comuns)

| Variável fora do padrão | Padrão correto |
|-------------------------|----------------|
| `DATABASE_URL` | `DATABASE_HOST` + `DATABASE_PORT` + ... |
| `REDIS_ADDR` | `CACHE_URI` |
| `REDIS_PASSWORD` | `CACHE_PASSWORD` |
| `DB_HOST`, `DB_USER` | `DATABASE_HOST`, `DATABASE_USER` |
| `RABBIT_HOST` | `MESSAGING_HOST` |
| `MQ_USER` | `MESSAGING_USER` |

---

## Quando uma variável nova é legítima

Algumas variáveis são genuinamente fora do padrão porque pertencem a integrações externas específicas que o gofi-sdk não gerencia:

- Credenciais de IDP externo: `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REDIRECT_URI`
- Secrets de JWT customizados: `JWT_SECRET` (quando não usa o provider padrão do gofi/iam)
- Chaves de APIs de terceiros: `STRIPE_API_KEY`, `SENDGRID_API_KEY`, etc.

**Regra:** se a variável é para um SDK/serviço externo não coberto pelo gofi-sdk, ela pode existir — mas deve ser confirmada com o dev e documentada explicitamente na spec (§ Variáveis de Ambiente).

---

## Protocolo de confirmação

Quando qualquer agent identificar uma variável de ambiente não catalogada no padrão:

1. **NÃO use a variável fora do padrão silenciosamente**
2. Pergunte ao dev:
   > "A variável `{VAR_NAME}` não faz parte do padrão gofi. Confirma que devemos usá-la? Se sim, para qual propósito?"
3. Com a confirmação, documente na spec e, se for um padrão recorrente, atualize `gofi-sdk/config.md`
