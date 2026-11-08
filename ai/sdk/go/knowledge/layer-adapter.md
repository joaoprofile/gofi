---
name: layer-adapter
description: Resumo da camada adapter em Go — ports do IAM, implementação única ou bridge/factory, adapter HTTP sobre netx/httpx, classificação de erro e regras de dependência
sdk: v0.2.18
keywords: [camada, adapter, port, UserPort, TenantPort, SessionPort, bridge, factory, httpx, NewRequest, RegisterExternalError, classifier, composition root]
---

# Camada Adapter — resumo

Leia este primeiro; o documento completo só na situação da última seção.

## O que faz e o que não faz

**Faz:** é a borda entre um SDK ou API externa e o domain model: implementa um
port, chama o externo, mapeia o wire format e classifica o erro.

**Não faz:** regra de negócio; persistência interna (é "service disfarçado");
hidratar tenancy (é do service); enriquecimento em N+1; instrumentar a si mesmo
(decorator na factory).

## Qual forma usar

| Situação | Onde | Forma |
|---|---|---|
| Uma implementação, sem troca prevista | `{pathService}/domain/{contexto}/adapter/` | sem bridge, sem factory |
| Usuários/tenants no banco do projeto | idem (`iam_adapter.go`) | `port.UserPort` + `port.TenantPort` sobre o repository |
| N implementações, escolha em runtime | `{pathService}/adapter/{tech}/{contexto}/` | bridge + factory + `application/` |

Na dúvida, implementação única: só extraia a bridge quando o segundo adapter
aparecer de verdade. Dentro de `repository/`, nunca.

## Port do IAM

Assinatura é a do port, com `error` do SDK:

```go
func (a *userAdapter) FindByEmail(ctx context.Context, email string) (*types.User, error) {
	u, err := a.repo.FindByEmail(ctx, email)
	if err != nil || u == nil {
		return nil, core.ErrInvalidCredentials // inexistente = mesma resposta de senha errada
	}
	return toIAMUser(u), nil
}

func (a *userAdapter) AssertAccess(ctx context.Context, userID, tenantID, module string) error
```

- O IAM sobe pelo componente `gofi/component/iam`. `iam.New` com port próprio
  só onde o default não serve.
- **`SessionPort` próprio** só com storage já existente: `Save` nunca persiste
  token cru; `Revoke` marca, não apaga; `Get` inexistente →
  `core.ErrSessionNotFound`; implemente `RevokeIfActive` de forma atômica.
- `iam.New` não deriva `TenantTicketSecret`: defina-o.
- Claim de domínio não é caso de adapter: use `SelectTenantInput.Extra`.
- Método que "não se aplica" devolve erro explícito, nunca `nil` silencioso ou
  `panic`.

## Adapter HTTP

```go
const (
	{tech}OpPath      = "/some/path/%s" // sempre relativo ao BaseURL
	bridgeHTTPTimeout = 5 * time.Second
)

var Err{Tech}OpTransient = errs.RegisterExternalError("{TECH}_{CONTEXTO}_OP_TRANSIENT", "...")

type opResponse struct { // wire DTO privado, não vaza
	ID string `json:"id"`
}

func (b *bridge) Op(ctx context.Context, tkn *token.Token, in {contexto}Bridge.OpInput) (*{contexto}Bridge.OpResult, errs.AppError) {
	req := httpx.NewRequest[opResponse](ctx, b.client, http.MethodPost, fmt.Sprintf({tech}OpPath, in.ExternalID))
	req.SetHeader("Authorization", "Bearer "+tkn.AccessToken)
	req.SetBody(&opBody{})
	resp, err := req.Execute()
	if err != nil {
		return nil, classifyOpError(err) // httpx.FromError(err).Status → AppError
	}
	// resp vem nil, sem erro, em 204 ou sem Content-Type JSON: cheque antes de usar
	return &{contexto}Bridge.OpResult{ExternalID: resp.ID}, errs.AppError{}
}
```

Cliente: `httpx.NewClient(&httpx.HttpClientConfig{Name, BaseURL, Timeout, Retries, RetrySleep, RateLimit})`.

## Regras que mais causam erro

- **Dependência:** `bridge/` e `factory/` não importam `adapter/`. O adapter
  importa só `domain/{contexto}/{bridge,model}` e libs externas; nunca
  `service/` nem `repository/`. (Adapter de port do IAM é implementação única
  no contexto e consome o repository.)
- **Registro na factory só no composition root** (`wire.go`). Sem `init()`.
- **Factory cacheia 1 instância por key**, com `sync.Map` + `LoadOrStore`.
  Decorators antes de cachear. Sem cache, cada `Get()` cria client com rate
  limiter próprio e estoura o limite real.
- **Construtor devolve a interface do domain.** Falha ao criar o client volta
  como `error`: só o `main` encerra o processo (`absolute-rules.md` #16), ainda
  que o template HTTP mostre `logging.Fatal`.
- **Retorno HTTP:** `(*Result, errs.AppError)`, com status classificado **no
  adapter**, por erros `{TECH}_{CONTEXTO}_{OP}_{TRANSIENT|PERMANENT|AUTH|RATE_LIMITED}`.
- **Input/Output moram em `bridge/`**; o wire DTO é privado e fica no adapter.
- **Token é argumento separado**, nunca campo do `Input`.
- **`NewRequest` recebe só path relativo:** `BaseURL` + URL absoluta duplica o
  host e rende `405`/`404`.
- **Encoding vem do tipo do body:** `url.Values` → form-urlencoded; demais →
  JSON. Header manual não troca.
- **`POST`/`PATCH` só retentam com `Idempotency-Key`.** `429` esgotado volta
  como `408`: trate como rate limit.
- **Método sem suporte** devolve `Err{Contexto}BridgeNotSupported`; o consumer
  dá `msq.Ack`, não `Nack`.
- **Teste:** mapper sem rede; em `httptest`, sempre
  `Content-Type: application/json` antes do `Write`.

## Quando abrir o detalhe

| Documento | Abra quando |
|---|---|
| `.claude/sdk/go/knowledge/bridge-factory-adapter-pattern.md` | houver N implementações ou for escrever adapter HTTP completo |
| `.claude/sdk/go/knowledge/application-bridge-rules.md` | for decidir entre service e application |
| `.claude/sdk/go/knowledge/iam.md` | for implementar `UserPort`/`TenantPort` |
| `.claude/sdk/go/knowledge/iam-adapter-pattern.md` | for implementar `SessionPort`, `AuthPort` ou `TokenPort` |
| `.claude/sdk/go/knowledge/http-client-signing.md` | a API externa exigir assinatura |
| `.claude/sdk/go/knowledge/bucket-storage.md` | a integração for com bucket |
| `.claude/sdk/go/knowledge/messaging-msq.md` | a integração for com mensageria |
| `.claude/sdk/go/api/iam-port.md` | precisar da assinatura exata de um port |
| `.claude/sdk/go/api/netx-httpx.md` | precisar de opção do client HTTP |
