# netx/awssign

`import "github.com/joaoprofile/gofi-sdk-go/netx/awssign"` · SDK gofi/v0.2.4 (310ba1a)

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package awssign signs httpx client requests with AWS Signature V4 (API Gateway IAM
auth, OpenSearch, Lambda URLs, ...). Credentials resolve through base/cloud/aws,
so IRSA / Pod Identity work without static keys.

## Constantes

```go
const ServiceExecuteAPI = "execute-api"
```

ServiceExecuteAPI is the signing name of API Gateway.

## awssign.Config

```go
type Config struct {
	// Service is the SigV4 signing name; defaults to ServiceExecuteAPI.
	Service string
	AWS     cloudaws.Config
}
```

Config selects the target service and the AWS identity.

## awssign.Signer

```go
type Signer struct {
	// contains filtered or unexported fields
}
```

Signer implements httpx.Signature.

### awssign.New

```go
func New(ctx context.Context, cfg Config) (*Signer, error)
```

New resolves credentials and region for cfg.

### awssign.NewWithConfig

```go
func NewWithConfig(awsCfg awssdk.Config, service string) (*Signer, error)
```

NewWithConfig builds a Signer from an existing aws.Config.

### Signer.Sign

```go
func (s *Signer) Sign(req *http.Request, body []byte) (*http.Request, error)
```

Sign adds the SigV4 headers, covering every header already set on req.
body must be the exact payload sent; when nil, the payload is read from
req.GetBody, and a body that cannot be replayed is an error rather than a
signature over the wrong payload.

