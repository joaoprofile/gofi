---
name: bucket-storage
description: Object storage com gofi base/bucket — provider por import, bucket.Open no composition root, wrapper de domínio, presign e testes com mem
sdk: v0.8.2
keywords: [bucket, object-storage, s3, minio, oci, presign, upload, file-service, BUCKET_PROVIDER]
---

# Object storage — enviar arquivos para bucket via `base/bucket`

Padrão para subir/baixar arquivos em object storage (S3/MinIO/R2, OCI Object
Storage, diretório local) pela abstração provider-agnóstica do SDK. Vale para
qualquer contexto que gera um artefato (planilha bulk, relatório, anexo,
snapshot exportado) e precisa persistir os bytes fora do banco.

Referência gerada: `.claude/sdk/go/api/base-bucket.md` (+ `base-bucket-s3.md`,
`base-bucket-oci.md`, `base-bucket-file.md`, `base-bucket-mem.md`,
`base-bucket-buckettest.md`); env → config em `gofi-config.md` § `config.Bucket`.

---

## Abstração do SDK (o que você importa)

Domínio e wrapper importam só **`github.com/gofi-labs/gofi-sdk-go/base/bucket`**.

```go
type PutInput struct {
    Key         string    // nome do objeto (obrigatório)
    Body        io.Reader // conteúdo (obrigatório)
    Size        int64     // content-length; negativo se desconhecido
    ContentType string    // MIME opcional
}

type Store interface {
    Put(ctx, in PutInput) error                                    // sobrescreve a mesma key
    Get(ctx, key string) (Object, io.ReadCloser, error)            // caller fecha; ErrNotFound se não existe
    List(ctx, prefix string) ([]Object, error)                     // nunca nil
    Delete(ctx, key string) error                                  // idempotente
    PresignGet(ctx, key string, ttl time.Duration) (string, error) // URL temporária de download direto
}
```

Sentinelas: `bucket.ErrNotFound`, `bucket.ErrInvalidConfig`. Listagem grande
sem carregar tudo: `for obj, err := range bucket.All(ctx, store, prefix)`.

---

## Provider — escolhido por import + `BUCKET_PROVIDER`

Cada backend se registra no `init()` via `bucket.Register`. Só o importado
entra no binário.

| `BUCKET_PROVIDER` | Import (blank, só no composition root) | Uso |
|---|---|---|
| `s3` (alias `minio`) | `_ ".../base/bucket/s3"` | AWS S3, MinIO, R2 — credenciais via `base/cloud/aws` |
| `oci` | `_ ".../base/bucket/oci"` | OCI Object Storage — credenciais via `base/cloud/oci` |
| `file` | `_ ".../base/bucket/file"` | diretório local (`BUCKET_ENDPOINT` = diretório); dev/nó único |
| `mem` | `_ ".../base/bucket/mem"` | memória; testes |
| `none` / vazio | — | feature desligada (`bucket.Open` devolve `ErrInvalidConfig`) |

**Não existem** `config.OpenBucket`/`config.OpenBucketFromEnv`: o caminho é
`bucket.Open(ctx, config.Bucket(env))` (ou `bucket.OpenURL(ctx, "s3://…")`).
Provider não importado → `bucket.Open` falha com `ErrInvalidConfig` citando o
import que falta.

---

## Composition root — abrir o Store uma vez, tolerar `nil`

O `Store` é infra: abre-se **uma vez** no `wire.go`/`main.go` e injeta-se por
parâmetro. **Nunca** ler env dentro do domínio.

```go
// wire.go — package main
import (
    "github.com/gofi-labs/gofi-sdk-go/base/bucket"
    _ "github.com/gofi-labs/gofi-sdk-go/base/bucket/s3" // ou .../oci — o do BUCKET_PROVIDER
    "github.com/gofi-labs/gofi-sdk-go/base/environment"
    "github.com/gofi-labs/gofi-sdk-go/gofi/config"
)

func buildBucketStore(ctx context.Context, env *environment.Environment) bucket.Store {
    cfg := config.Bucket(env) // BUCKET_* → bucket.Config
    if !cfg.IsConfigured() {
        return nil // BUCKET_PROVIDER vazio/none: feature off
    }
    store, err := bucket.Open(ctx, cfg)
    if err != nil {
        logging.Warn("object storage unavailable; file features disabled", slog.Any("error", err))
        return nil // degradação graciosa — não fataliza o boot
    }
    return store
}
```

`env` vem de `svc.Environment()` (serviço do `gofi.New(...).Build()`) ou
`environment.Instance()`.

> **Bucket é feature opcional, não dependência dura.** Store indisponível →
> `nil`, o serviço sobe, e só as operações de arquivo degradam (nil-check no
> wrapper). Bucket fora **não** derruba serviço cujo core é outra coisa.

`bucket.Open` de S3/OCI não faz I/O de rede (OCI resolve o namespace no 1º
uso) — credencial errada aparece na primeira operação, não no boot.

---

## Wrapper de domínio — não espalhar `bucket.Store` cru pelo código

O domínio **não** chama `store.Put` em N lugares. **Um** serviço de arquivo por
contexto (`{ctx}/storage/{ctx}_file_service.go`) encapsula: (a) nil-check do
store, (b) layout da object key, (c) TTL do presign, (d) tradução de erro para
`errs.AppError` do contexto. O resto do domínio só enxerga
`Store(...) (key, err)` / `DownloadURL(...)` / `Delete(...)`.

```go
package storage

type {Ctx}FileService interface {
    Store(ctx context.Context, meta model.FileMeta, body io.Reader, size int64) (string, errs.AppError) // devolve a object key
    DownloadURL(ctx context.Context, id uuid.UUID, tenant string) (model.FileLink, errs.AppError)
    Delete(ctx context.Context, id uuid.UUID, tenant string) errs.AppError
}

type {ctx}FileService struct {
    store bucket.Store
    repo  repository.Repository
    ttl   time.Duration
}

func New{Ctx}FileService(store bucket.Store, repo repository.Repository, ttl time.Duration) {Ctx}FileService {
    return &{ctx}FileService{store: store, repo: repo, ttl: ttl} // ttl vem da config do serviço
}

func (s *{ctx}FileService) Store(ctx context.Context, meta model.FileMeta, body io.Reader, size int64) (string, errs.AppError) {
    if s.store == nil {
        return "", service.Err{Ctx}FileStore.New() // store nil = feature off → erro estável, nunca panic
    }
    key := objectKey(meta)
    if err := s.store.Put(ctx, bucket.PutInput{Key: key, Body: body, Size: size, ContentType: meta.ContentType}); err != nil {
        return "", service.Err{Ctx}FileStore.Wrap(err)
    }
    return key, errs.AppError{}
}

// Object key: prefixo estável + discriminadores de tenancy/identidade + nome do arquivo.
// A key retornada é o handle do objeto: persistir na linha do registro.
func objectKey(meta model.FileMeta) string {
    return path.Join("{prefixo}", meta.Type, meta.Tenant, meta.ID.String(), meta.FileName)
}
```

`DownloadURL` resolve a key persistida e chama `s.store.PresignGet(ctx, key, s.ttl)`;
`Delete` chama `s.store.Delete(ctx, key)`. Ambos com o mesmo nil-check;
`errors.Is(err, bucket.ErrNotFound)` vira o erro "não encontrado" do contexto.

> `PresignGet` de `file` devolve `file://` (só legível no host) e de `mem`
> devolve `mem://` (não baixável) — URL de download real só com S3/OCI.

---

## Receita de upload

```go
data, err := io.ReadAll(file) // consome o io.Reader de entrada uma vez
// ... valida/parseia data ...
key, appErr := fileSvc.Store(ctx, meta, bytes.NewReader(data), int64(len(data)))
```

- **Body re-legível + Size exato.** `bytes.NewReader(data)` e `int64(len(data))` —
  não o `io.Reader` original (já drenado) nem `-1` quando o tamanho é conhecido.
  Size negativo só em streaming de tamanho desconhecido.
- **Persistir a key retornada** na linha do registro (coluna `path_file` /
  `object_key`) — é o handle para `PresignGet`/`Delete` depois.

### Armadilha — body consumido uma vez

`io.Reader`/`*bytes.Buffer` drena na primeira leitura. Se o **mesmo** arquivo vai
para dois destinos (ex.: API externa **e** bucket), leia para `[]byte` **uma
vez** e crie um `bytes.NewReader(data)` **por destino**. Reusar o mesmo
`*bytes.Buffer` faz o segundo receber corpo vazio, sem erro.

```go
data, _ := io.ReadAll(file)
_ = externalAPI.Upload(ctx, bytes.NewReader(data))                            // 1º reader
key, _ := fileSvc.Store(ctx, meta, bytes.NewReader(data), int64(len(data))) // 2º reader
```

---

## Testes

- **Wrapper/service:** injete `mem.New("test")` (`base/bucket/mem`) — Store real
  em memória, sem mock handcraft de 5 métodos. Para o caminho "feature off",
  injete `nil`.
- **Store próprio** (implementação nova de `bucket.Store`): rode
  `buckettest.Run(t, store)` — contrato que todo backend do SDK cumpre.

---

## Variáveis de ambiente (modeladas pelo SDK — `BUCKET_*`)

Campos do `environment.Environment`, mapeados por `config.Bucket(env)` — **não**
são vars fora do padrão. Tabela completa em `env-vars-standard.md`.

```
BUCKET_PROVIDER=s3                        # s3 | minio | oci | file | mem | none
BUCKET_NAME=<bucket>
BUCKET_REGION=<region>
BUCKET_ENDPOINT=<url>                     # MinIO/R2/LocalStack; diretório no provider file
# S3: BUCKET_S3_ACCESS_KEY / BUCKET_S3_SECRET_KEY / BUCKET_S3_USE_SSL
#     chaves vazias → cadeia AWS padrão (IRSA, EKS Pod Identity, instance profile)
# OCI:
BUCKET_OCI_NAMESPACE=<namespace>          # vazio → resolvido no 1º uso e cacheado
BUCKET_OCI_AUTH_MODE=workload_identity    # api_key (default) | instance_principal | resource_principal | workload_identity
# api_key exige BUCKET_OCI_TENANCY_ID / _USER_ID / _FINGERPRINT / _PRIVATE_KEY (/ _PASSPHRASE)
```

Em cluster prefira identidade da máquina/pod (IRSA/Pod Identity na AWS;
`instance_principal`/`workload_identity` na OCI) — sem chave no `.env`. Detalhes
em `cloud-identity.md`.

---

## Anti-padrões

- **Importar `bucket/s3`, `bucket/oci`… fora do composition root**, ou chamar
  `s3.New`/`oci.New` no domínio. O resto depende de `bucket.Store` (interface).
- **Ler `BUCKET_*` no domínio.** Env só no composition root; o wrapper recebe o
  `Store` pronto.
- **`store.Put` cru espalhado.** Encapsule no `{Ctx}FileService`.
- **Fatalizar boot quando o bucket não abre.** `nil` store + feature degradada.
- **Reusar o mesmo `io.Reader`/`*bytes.Buffer` em dois uploads.**
- **Não persistir a object key.** Sem a key não há download nem delete depois.
- **Esquecer `Close()` no `ReadCloser` de `Get`.**
