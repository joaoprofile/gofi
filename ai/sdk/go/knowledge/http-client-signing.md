---
name: http-client-signing
description: Cliente HTTP netx com assinatura de request — netx.Signature, AWS SigV4 via netx/awssign, assinador próprio (HMAC), retry e idempotência
sdk: v0.8.2
keywords: [netx, NewClient, NewRequest, SetSignature, Signature, awssign, SigV4, execute-api, HMAC, retry, Idempotency-Key, HttpError]
---

# Cliente HTTP e assinatura — `netx.NewRequest` + `netx.Signature`

Referência: `.claude/sdk/go/api/netx.md` (§`HttpClient`, `Request`, `Signature`),
`netx-awssign.md`. Credencial AWS: `cloud-identity.md`.

## Chamada tipada

```go
client, err := netx.NewClient(&netx.HttpClientConfig{
    Name: "<partner>", BaseURL: baseURL, Timeout: 10 * time.Second,
    Retries: 2, RetrySleep: 200 * time.Millisecond,
})
// client criado uma vez (composition root / construtor do adapter), nunca por request

req := netx.NewRequest[model.RemoteEntity](ctx, client, http.MethodPost, "/entities")
req.SetBody(payload)
req.SetHeader("Idempotency-Key", key) // habilita retry seguro em POST
created, err := req.Execute()         // *T; nil sem erro em 204 ou resposta não-JSON
if err != nil {
    var httpErr *netx.HttpError
    if errors.As(err, &httpErr) && httpErr.Status == http.StatusNotFound {
        return nil, ErrRemoteNotFound.New()
    }
    return nil, ErrRemoteCall.Wrap(err) // RegisterExternalError → 502
}
```

- Retry só onde repetir não duplica efeito: GET/HEAD/OPTIONS/PUT/DELETE, ou
  qualquer método com `Idempotency-Key`. 5xx, erro de rede e 429 (respeita
  `Retry-After`). `DisableRetryOn429` quando um rate limiter externo dita o ritmo.
- `Request.ResponseHeaders` expõe os headers da última resposta (ex.: limite
  informado pelo parceiro).
- Erro de parceiro vira `errs.RegisterExternalError` no service/application
  (502), não `RegisterOperation`.

## Assinatura

`netx.Signature` é uma interface; o cliente chama `Sign(req, body)` **a cada
tentativa** (retry leva assinatura e timestamp novos), **antes** de aplicar os
headers de `SetHeader` — header que precisa entrar na assinatura é o
assinador quem põe. Erro de assinatura aborta (`signature request error`).

### AWS SigV4 — `netx/awssign`

```go
signer, err := awssign.New(ctx, awssign.Config{})               // API Gateway (execute-api), cadeia default
// awssign.New(ctx, awssign.Config{Service: "es"})               // OpenSearch
// awssign.NewWithConfig(awsCfg, awssign.ServiceExecuteAPI)      // reusa aws.Config existente
req.SetSignature(signer)
```

- Credencial pela cadeia de `base/cloud/aws` (IRSA / Pod Identity, sem chave
  estática). Módulo separado: só linka o SDK AWS quem importa `netx/awssign`.
- Um `*awssign.Signer` por serviço-alvo, criado uma vez.

### Assinador próprio (webhook HMAC, parceiro)

```go
type hmacSigner struct{ key []byte }

func (s hmacSigner) Sign(req *http.Request, body []byte) (*http.Request, error) {
    mac := hmac.New(sha256.New, s.key)
    mac.Write(body) // body é exatamente o payload enviado
    req.Header.Set("X-Signature", hex.EncodeToString(mac.Sum(nil)))
    return req, nil
}
```

Chave vem de secret reference (`secret://…`), nunca literal nem log.

## Anti-padrões

- ❌ `http.Client`/`http.NewRequest` cru num serviço gofi — perde retry, rate limit, trace.
- ❌ Assinar dentro de um middleware/`SetHeader` fixo — o timestamp envelhece nos retries.
- ❌ POST com `Retries > 0` sem `Idempotency-Key` esperando retry — não haverá.
- ❌ `netx.NewClient` por request.
