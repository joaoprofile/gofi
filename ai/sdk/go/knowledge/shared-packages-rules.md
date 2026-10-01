---
name: shared-packages-rules
description: Regras de pacotes comuns em Go — helpers, money, object storage (base/bucket), e-mail (base/mail), identidade de nuvem, observabilidade
sdk: v0.8.2
keywords: [helpers, money, currency, bucket, object-storage, presign, mail, smtp, cloud, credentials, observability, otel]
---

# Regras — pacotes comuns (helpers, money, bucket, mail, observabilidade)

Índice; detalhe no arquivo apontado. `helpers` e `money` são pacotes **do
projeto** (`services/common/…`); `bucket`, `mail`, `cloud` e `obs` são do SDK
(`github.com/joaoprofile/gofi-sdk-go/...`, referência em `.claude/sdk/go/api/`).

- **Helpers reaproveitáveis ficam em `services/common/helpers/` (genérico) ou no
  domínio (model-bound).**
  - **Genérico, não-domínio** → `services/common/helpers/`: `TruncateString(s, max)`,
    `ParseFloatLoose`, `ParseIntLoose`, `Chunk[T]`,
    `StrPtr/StrPtrOrNil/StrFromPtr/FloatOrZero/IntToInt32`, `JSONStringMap`.
    **Critério:** a função **não** depende de tipo do domínio.
  - **Model-bound, compartilhado entre adapters** → `services/domain/{ctx}/model/`.
    **Critério:** opera sobre tipo do domínio **e** é reusado por ≥2 adapters.
  - **Anti-padrão:** helper privado replicado em N adapters. No **segundo**
    adapter que precisar, **promover** (não duplicar).
- **Valor monetário, moeda e país → `services/common/money` (pacote canônico do
  projeto).** Todo parse/format de valor, moeda ou resolução país→moeda passa
  por `money` — **nunca** `strconv.ParseFloat` cru em string de dinheiro,
  **nunca** hardcode de símbolo/casas/separador, **nunca** mapa local
  país→moeda. `money.Currency{Code, Decimals, Symbol, DecimalSep, GroupSep}` +
  catálogo de **todos os países LatAm**.
  - **Parse:** `money.ParseLoose(raw)` quando a moeda é **desconhecida no parse**
    (entrada multi-país; detecção estrutural: último `.`/`,` com 1–2 dígitos é o
    decimal — LatAm tem ≤2 casas —, 3 dígitos atrás = milhar; cobre `1.234,56`
    (BRL/ARS/COP…), `1,234.56` (MXN/PEN/USD) e sem-centavos `1.234.567`→1234567
    (CLP/PYG)). Moeda **conhecida** → `Currency.Parse(raw)` /
    `Currency.Format(v)`.
  - **Arredondamento** na borda via `Currency.Round/Truncate` (respeita
    `Decimals`; CLP/PYG têm 0). Math interno em `float64`.
  - **Lookups:** `money.ByCode`, `money.ByCountry`, `money.CodeForCountry`
    (fallback USD), `money.IsValidCode`.
  - `services/common/integration.Currency` é só alias de compat — código novo
    importa `money`. DTOs expõem `currencyCode` (ISO 4217) / `countryCode`
    (ISO 3166-1), nunca o struct.
- **Enviar arquivo a bucket (object storage) → `base/bucket` via wrapper de domínio.**
  Artefato que persiste bytes fora do banco (planilha bulk, relatório, anexo,
  snapshot) sobe por `bucket.Store`, aberto **uma vez** no composition root com
  `bucket.Open(ctx, config.Bucket(env))` + blank import do provider
  (`base/bucket/s3` ou `base/bucket/oci`; `file`/`mem` para dev/teste) e
  injetado — `nil` = feature off, nunca fataliza o boot. **Não existe**
  `config.OpenBucket*`. O domínio **não** chama `store.Put` cru nem importa
  provider: encapsula num `{Ctx}FileService` (`{ctx}/storage/`) com nil-check +
  object key (`path.Join(prefixo, tipo, tenant, id, filename)`) + tradução para
  `errs.AppError`, e devolve a **key** (persistir na linha para
  `PresignGet`/`Delete`). Upload: `PutInput{Key, Body: bytes.NewReader(data),
  Size: int64(len(data)), ContentType}`; mesmo arquivo para 2 destinos = um
  `bytes.NewReader` **por destino**. Testes com `mem.New(...)`. Vars `BUCKET_*`
  são modeladas pelo SDK. Detalhe: `bucket-storage.md`.
- **E-mail → `base/mail` atrás de wrapper de domínio.** `config.NewMailer(env)`
  uma vez no composition root; `mail.ErrNotConfigured` = feature off (`nil`),
  config inválida = falha de boot. Templates registrados no boot
  (`mail.NewTemplateEngine` + `Register`), envio por `Send`/`SendBulk`
  (checar `BulkResult.Failed`). Sem `net/smtp` direto. Detalhe: `mail.md`.
- **Credencial de nuvem → `base/cloud/aws` / `base/cloud/oci`.** Bucket, fila,
  secrets e assinatura resolvem identidade por eles; em cluster, identidade do
  pod/máquina (IRSA/Pod Identity; `workload_identity`/`instance_principal`),
  nunca chave estática em produção. Detalhe: `cloud-identity.md`.
- **Observabilidade via `obs` (OpenTelemetry).** Padrão completo (lazy init,
  classifier centralizado, decorator para interfaces, ResetForTesting) em
  `observability-otel.md`. Logs estruturados (`obs/logging`, slog) já seguem
  para o backend quando o componente `observability` está ativo — não duplicar
  log shipping. Mensageria já emite spans/métricas pelo pipeline do `msq`.
  **Onde o pacote mora:** `domain/{ctx}/observability/` por padrão (`attrs`,
  `outcomes`, nomes de métrica são vocabulário de domínio); sobe para
  `common/observability/{ctx}/` **só** quando (a) pacote em `common/` precisa
  gravar nele, ou (b) instrumenta capacidade transversal. Heurística:
  `gofi show <pacote>` lista quem depende — importador em `common/` → pacote em
  `common/`. Regra em `observability-otel.md` §"Onde o pacote mora".
