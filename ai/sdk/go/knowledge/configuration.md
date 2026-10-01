---
name: configuration
description: Configuração no gofi — base/environment, .env só fora de prod/stage, X_FILE, referências secret:// (env, file, awssm, ocivault), adapters gofi/config, Config tipado do projeto e timezone UTC
sdk: v0.8.2
keywords: [environment, config, .env, GOFI_DOTENV, X_FILE, secret://, secrets, awssm, ocivault, ParseStructAnnotationFunc, LoadConfig, TIMEZONE, timezone, UTC, os.Getenv]
---

# Configuração — ambiente, segredos e `Config` tipado

API: `.claude/sdk/go/api/base-environment.md`, `gofi-config.md`,
`gofi-config-core.md`, `base-secrets*.md`, `base-timezone.md`,
`base-common.md` (`ParseStructAnnotationFunc`). Nomes das variáveis:
`env-vars-standard.md`.

## As três camadas

| Camada | Quem | Lê o ambiente? |
|---|---|---|
| Biblioteca (`sqln`, `msq`, `netx`, `iam`, `obs`, `base/mail`, `base/bucket`…) | recebe um `Config` **tipado e explícito** | **Nunca** |
| Módulo `gofi` (`gofi/config`, `gofi/config/core`, `gofi/component/*`) | mapeia `*environment.Environment` → `Config` de cada biblioteca | Sim — é o único |
| Projeto (`pathCmd`) | `config.go` monta o `Config` do serviço a partir do `Environment` | Só em `config.go` |

Consequência: domínio (`handler`/`service`/`repository`/`adapter`) **nunca**
lê variável de ambiente — recebe valores prontos por construtor.

## Carga — `base/environment`

`environment.Instance()` (singleton, chamado pelo `Build`) faz, uma vez:

1. **`.env` só fora de produção.** Lido quando `APP_ENVIRONMENT` não é `prod`
   nem `stage`; procurado subindo a partir do diretório de trabalho. Não
   sobrescreve variável já definida no processo. `GOFI_DOTENV=true|false`
   força ligado/desligado.
2. Cada variável `X` aceita **`X_FILE`** = caminho de arquivo (segredo montado
   em volume); o conteúdo vira o valor.
3. Valor `secret://<provider>/<nome>[#chave-json]` é **resolvido no boot**
   (ver abaixo).
4. Erros não dão panic: ficam em `environment.LoadError()` e o `Build` os
   devolve junto com os demais.

`APP_ENVIRONMENT` válido: `dev`, `stage`, `test`, `prod`
(`environment.ENV_*`). `dev` = log em texto; qualquer outro = JSON.

Helpers do `Environment` para quem precisa (sempre em `config.go`):
`env.Auth()` (TTLs default 15m/168h, issuer = `APP_NAME`), `env.OAuth()`
(`Google.IsConfigured()`), `env.HTTP()` (`PORT`, `ALLOWED_ORIGINS` já em
`[]string`), `env.Observability()`, `env.RequireAuth()`,
`env.RequireGoogleOAuth()`, `env.IsDatabaseConfigured()` e afins.

## Segredos — `secret://`

```bash
DATABASE_PASSWORD=secret://awssm/prod/{servico}/db#password
JWT_SECRET=secret://ocivault/<vault-ocid>/<nome-do-segredo>
CACHE_PASSWORD=secret://file/run/secrets/cache-password   # absolute path without the leading slash
MAIL_PASSWORD_FILE=/run/secrets/mail                        # X_FILE alternative
```

| Provider | Como habilita | Identidade |
|---|---|---|
| `env`, `file` | embutidos | — |
| `awssm` | `import _ "github.com/joaoprofile/gofi-sdk-go/base/secrets/awssm"` | cadeia padrão AWS (`AWS_REGION`, IRSA / EKS Pod Identity, roles) |
| `ocivault` | `ocivault.Register(ocivault.Config{Credentials: cloudoci.Config{AuthMode: cloudoci.AuthWorkloadIdentity}})` no `main`, **antes** do `Build` | OCI não tem cadeia implícita: modo explícito (`api_key`, `instance_principal`, `resource_principal`, `workload_identity`) |

- `#chave` extrai um campo de segredo JSON; chave ausente = erro
  (`secrets.ErrNotFound`).
- Resolução só acontece **no boot** (timeout 30s por segredo) e cada segredo é
  buscado uma vez. Rotação exige restart.
- Referência inválida ou provider não importado falha o `Build` — nunca sobe
  com senha vazia.
- Fora do fluxo de ambiente (valor vindo de outro lugar):
  `secrets.Resolve(ctx, ref)` ou um `secrets.NewResolver()` reutilizado.

**Regra:** segredo nunca em texto no manifesto/`values`/pipeline — sempre
`secret://` ou `X_FILE`. `.env` com segredo real não é commitado.

## Adapters `gofi/config` — biblioteca fora do orquestrador

Para recursos sem componente, o `Config` da biblioteca sai do adapter, nunca
de leitura manual:

| Adapter | Produz |
|---|---|
| `config.Bucket(env)` | `bucket.Config` → `bucket.Open(ctx, cfg)` (import `_ ".../base/bucket/s3"` ou `oci`) |
| `config.Mail(env)` / `config.NewMailer(env)` | `mail.Config` / `mail.Mailer` (`mail.ErrNotConfigured` sem `MAIL_HOST`/`MAIL_FROM_EMAIL`) |
| `config.IAM(env)` | `iam.DefaultConfig` (o componente `iam` já usa) |
| `config.Debug(env)` / `config.StartDebug(env)` | servidor pprof quando `SERVICE_DEBUG=true` |
| `database.ConfigFromEnv(env)` | `connection.Config` |
| `messaging.ConfigFromEnv(env)` | `msq.ProviderConfig` → `msq.Open(ctx, cfg)` |
| `observability.ConfigFromEnv(env)` | `obs.TeleConfig` |

O `env` vem de `svc.Environment()` (depois do `Build`), de `rt.Env()` (dentro
de `Start`) ou de `environment.Instance()` (antes do `Build`, mesmo singleton).
Em teste, `environment.Load()` devolve uma instância nova; entre casos,
`environment.ResetForTesting()`.

## `Config` do projeto — variáveis próprias

O `Environment` do SDK é fechado. Variável própria do serviço (horário de
cron, concorrência, feature flag) entra num struct **tipado** com tag `env`,
preenchido em `config.go` — único arquivo do projeto que toca o processo:

```go
type Config struct {
    AllowedOrigins []string // from env.HTTP()

    ReportEnabled    bool          `env:"REPORT_CRON_ENABLED"`
    ReportHour       int           `env:"REPORT_CRON_HOUR"`
    ReportLocation   string        `env:"REPORT_CRON_LOCATION"`
    OrderConcurrency int           `env:"ORDER_CONSUMER_CONCURRENCY"`
    ExportTimeout    time.Duration `env:"EXPORT_TIMEOUT"`
}

func LoadConfig(ctx context.Context, env *environment.Environment) (Config, error) {
    cfg := Config{ // defaults; an unset variable keeps its default
        AllowedOrigins:   env.HTTP().AllowedOrigins,
        ReportHour:       2,
        ReportLocation:   "UTC",
        OrderConcurrency: 4,
        ExportTimeout:    30 * time.Second,
    }
    resolver := secrets.NewResolver()
    var errs []error
    lookup := func(key string) string { // resolves secret:// in project variables too
        v, err := resolver.Resolve(ctx, os.Getenv(key))
        if err != nil {
            errs = append(errs, fmt.Errorf("%s: %w", key, err))
        }
        return v
    }
    if err := common.ParseStructAnnotationFunc(&cfg, "env", lookup); err != nil {
        errs = append(errs, err)
    }
    return cfg, errors.Join(append(errs, environment.LoadError())...)
}
```

- Chame **depois** de `environment.Instance()` (é ele que carrega o `.env`).
- Tipos aceitos pela tag: `string`, `bool`, `int`, `int64`, `float64`,
  `time.Duration` (e ponteiros). Lista/CSV: campo sem tag, preenchido à mão.
- Campo **do SDK nunca é redeclarado** (`JWT_SECRET`, `DATABASE_*`…): use o
  helper (`env.Auth()`, `env.HTTP()`) ou deixe o componente ler.
- `LoadConfig` devolve **todos** os erros juntos; validação de negócio
  (faixa de hora, fuso IANA) também aqui — ver `cronjob.md`.
- Nome da variável segue `env-vars-standard.md`.

## Timezone — UTC por padrão

- `Build` aplica `TIMEZONE` a `time.Local` antes de qualquer componente.
  **Vazio = UTC** (`timezone.Apply`; o comentário do campo
  `Environment.Timezone` que fala em default Brasil está desatualizado). Nome
  inválido falha o `Build`.
- O banco IANA vai **embutido** (`base/timezone` importa `time/tzdata`), então
  todo binário que usa `gofi.New` resolve `time.LoadLocation` em imagem
  `scratch`/distroless. Binário sem gofi que carrega fuso por nome importa
  `_ "time/tzdata"` no `main`.
- **Regra de negócio não depende de `time.Local`.** Horário de corte, "meia-
  noite", janela diária: fuso explícito (`time.LoadLocation(cfg.XxxLocation)`)
  vindo de config. `TIMEZONE` só muda a formatação padrão do processo.
- Persistência e APIs trafegam instantes (UTC / RFC 3339); conversão para fuso
  de exibição é da borda.
- Antes do `Build`, `time.Local` ainda é o do host — não calcule horário de
  negócio no `main` antes dele.

## Anti-padrões

- ❌ `os.Getenv` fora de `config.go` (handler, service, repository, adapter,
  `wire.go`).
- ❌ Biblioteca/pacote de domínio importando `base/environment`.
- ❌ Montar `Config` de biblioteca à mão com variáveis que o adapter já mapeia.
- ❌ Segredo em texto no manifesto; `.env` com segredo real no git.
- ❌ Depender do `.env` em `stage`/`prod` (não é lido lá).
- ❌ `ocivault.Register` depois do `Build` (os segredos já foram resolvidos).
- ❌ Confiar em `time.Local` para regra de negócio com horário.
