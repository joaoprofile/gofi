# Object storage — enviar arquivos para bucket via `gofi/base/bucket`

Padrão para subir/baixar arquivos em object storage (OCI Object Storage, S3/MinIO)
usando a abstração provider-agnóstica do gofi. Vale para qualquer contexto que
gera um artefato (planilha bulk, relatório, anexo, snapshot exportado) e precisa
persistir os bytes fora do Postgres.

---

## Abstração do SDK (o que você importa)

Import único: **`github.com/joaoprofile/gofi/base/bucket`**. Backends
(`bucket/oci`, `bucket/minio`) **nunca** são importados direto pelo projeto —
só pela factory `config`.

```go
type PutInput struct {
    Key         string    // nome do objeto no bucket (obrigatório)
    Body        io.Reader // conteúdo (obrigatório)
    Size        int64     // content-length; negativo se desconhecido
    ContentType string    // MIME opcional
}

type Store interface {
    Put(ctx, in PutInput) error
    Get(ctx, key string) (Object, io.ReadCloser, error)   // caller fecha o ReadCloser; ErrNotFound se não existe
    List(ctx, prefix string) ([]Object, error)
    Delete(ctx, key string) error                         // idempotente
    PresignGet(ctx, key string, ttl time.Duration) (string, error) // URL temporária de download direto
}
```

Sentinelas: `bucket.ErrNotFound`, `bucket.ErrInvalidConfig`.

---

## Composition root — abrir o Store uma vez, tolerar `nil`

O `Store` é infra: constrói-se **uma vez** no `wire.go`/`main.go` a partir do
environment e injeta-se por parâmetro. **Nunca** chamar env dentro do domínio.

```go
// wire.go — package main
func buildBucketStore() bucket.Store {
    store, err := config.OpenBucketFromEnv(environment.Instance())
    if err != nil {
        logging.Warn("object storage unavailable; file features disabled", slog.Any("error", err))
        return nil // degradação graciosa — não fataliza o boot
    }
    return store
}
```

`config.OpenBucketFromEnv(env)` (de `github.com/joaoprofile/gofi/config`) lê os
`BUCKET_*`, faz dispatch no provider e retorna o backend concreto. É o **único**
lugar que importa `oci`/`minio`.

> **Bucket é feature opcional, não dependência dura.** Se o storage não abre,
> `buildBucketStore` devolve `nil` e o serviço sobe mesmo assim — as operações
> de arquivo é que degradam (ver nil-check no wrapper). Um bucket indisponível
> **não** derruba um serviço cujo core é outra coisa (consumer, CRUD, engine).

---

## Wrapper de domínio — não espalhar `bucket.Store` cru pelo código

O domínio **não** chama `store.Put` direto em N lugares. Cria-se **um** serviço
de arquivo por contexto (`{ctx}/storage/{ctx}_file_service.go`) que encapsula:
(a) o nil-check do store, (b) o layout da object key, (c) o TTL do presign, (d)
a tradução de erro para `errs.AppError` do contexto. Assim o resto do domínio só
enxerga `Store(...) (key, err)` / `DownloadURL(...)` / `Delete(...)` e nunca
importa tipos do `bucket`.

```go
package storage

type {Ctx}FileService interface {
    Store(ctx, meta model.FileMeta, body io.Reader, size int64) (string, errs.AppError) // devolve a object key
    DownloadURL(ctx, id uuid.UUID, tenant string) (model.FileLink, errs.AppError)
    Delete(ctx, id uuid.UUID, tenant string) errs.AppError
}

type {ctx}FileService struct {
    store bucket.Store
    repo  repository.Repository
    ttl   time.Duration
}

func New{Ctx}FileService(store bucket.Store, repo repository.Repository) {Ctx}FileService {
    return &{ctx}FileService{store: store, repo: repo, ttl: helpers.EnvDuration("{CTX}_FILE_URL_TTL", 5*time.Minute)}
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
// Guarde a key retornada na linha do registro (coluna path_file/object_key) para
// depois fazer PresignGet/Delete — a key é o handle do objeto.
func objectKey(meta model.FileMeta) string {
    return path.Join("{prefixo}", meta.Type, meta.Tenant, meta.ID.String(), meta.FileName)
}
```

`DownloadURL` resolve a key persistida e chama `s.store.PresignGet(ctx, key, s.ttl)`;
`Delete` chama `s.store.Delete(ctx, key)`. Ambos com o mesmo nil-check.

O `{Ctx}FileService` injeta-se no service/application que produz o arquivo (ex.:
o bulk service que parseia a planilha), exatamente como qualquer outra dependência
do constructor.

---

## Receita de upload

```go
data, err := io.ReadAll(file)          // consome o io.Reader de entrada uma vez
// ... valida/parseia data ...
key, appErr := fileSvc.Store(ctx, meta, bytes.NewReader(data), int64(len(data)))
```

- **Body re-legível + Size exato.** Passe `bytes.NewReader(data)` (re-legível) e
  `int64(len(data))` — não o `io.Reader` original (já drenado) nem `-1` quando o
  tamanho é conhecido. Size negativo só quando genuinamente streaming de tamanho
  desconhecido (o backend bufferiza).
- **Persistir a key retornada** na linha do registro (coluna `path_file` /
  `object_key`) — é o handle para `PresignGet`/`Delete` depois.

### Armadilha — body consumido uma vez

`io.Reader`/`*bytes.Buffer` drena na primeira leitura. Se o **mesmo** arquivo vai
para dois destinos (ex.: upload multipart para uma API externa **e** para o
bucket), leia para `[]byte` **uma vez** e crie um `bytes.NewReader(data)` **por
destino**. Reusar o mesmo `*bytes.Buffer` nos dois faz o segundo receber corpo
vazio, sem erro.

```go
data, _ := io.ReadAll(file)
_ = externalAPI.Upload(ctx, bytes.NewReader(data))     // 1º reader
key, _ := fileSvc.Store(ctx, meta, bytes.NewReader(data), int64(len(data))) // 2º reader
```

---

## Variáveis de ambiente (modeladas pelo SDK — `BUCKET_*`)

Estão no `environment.Environment` do gofi e são mapeadas por `config.Bucket(env)`
— **não** são "vars fora do padrão", não precisam de confirmação. Tabela completa
em `env-vars-standard.md`.

```
BUCKET_PROVIDER=oci                       # oci | minio | none
BUCKET_NAME=<bucket>
BUCKET_REGION=<region>
BUCKET_OCI_NAMESPACE=<namespace>          # pré-semeado evita o GetNamespace lazy no 1º call
BUCKET_OCI_AUTH_MODE=instance_principal   # api_key | instance_principal | resource_principal | workload_identity
# api_key exige TENANCY_ID/USER_ID/FINGERPRINT/PRIVATE_KEY/PASSPHRASE;
# instance_principal/resource_principal/workload_identity dispensam key material.
# MinIO/S3: BUCKET_S3_ACCESS_KEY / BUCKET_S3_SECRET_KEY / BUCKET_S3_USE_SSL + BUCKET_ENDPOINT
```

Em cluster (OKE/instância OCI) prefira `instance_principal`/`workload_identity` —
a auth vem da própria máquina/pod, sem chave no `.env`.

---

## Anti-padrões

- **Importar `bucket/oci` ou `bucket/minio` no projeto.** Só `config.OpenBucket*`
  importa backend; o resto depende de `bucket.Store` (a interface).
- **Ler `BUCKET_*` no domínio.** Env só no composition root; o wrapper recebe o
  `Store` pronto.
- **`store.Put` cru espalhado.** Encapsule no `{Ctx}FileService` (nil-check +
  object key + erro de domínio num lugar só).
- **Fatalizar boot quando o bucket não abre.** `nil` store + feature degradada;
  o serviço sobe.
- **Reusar o mesmo `io.Reader`/`*bytes.Buffer` em dois uploads.** Um
  `bytes.NewReader(data)` novo por destino.
- **Não persistir a object key.** Sem a key não há download nem delete depois.
