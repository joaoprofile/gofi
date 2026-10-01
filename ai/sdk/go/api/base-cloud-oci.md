# base/cloud/oci

`import "github.com/joaoprofile/gofi-sdk-go/base/cloud/oci"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package oci resolves OCI credentials for every gofi integration (object
storage, queue, ...), so API keys and principals are configured once.

## Variáveis

```go
var ErrInvalidConfig = errors.New("oci: invalid credentials config")
```

ErrInvalidConfig is returned for missing or unsupported credential settings.

## Funções

### oci.ConfigurationProvider

```go
func ConfigurationProvider(cfg Config) (common.ConfigurationProvider, error)
```

ConfigurationProvider returns the OCI SDK provider for cfg.

## oci.AuthMode

```go
type AuthMode string
```

AuthMode selects where credentials come from. OCI never auto-detects the
principal, so the choice is explicit; empty means AuthAPIKey.

```go
const (
	// AuthAPIKey signs requests with a user API key (TenancyID, UserID, Fingerprint, PrivateKey).
	AuthAPIKey AuthMode = "api_key"
	// AuthInstancePrincipal uses the compute instance identity (metadata service).
	AuthInstancePrincipal AuthMode = "instance_principal"
	// AuthResourcePrincipal uses resource-principal variables (Functions, Container Instances).
	AuthResourcePrincipal AuthMode = "resource_principal"
	// AuthWorkloadIdentity uses the OKE pod's service-account identity.
	AuthWorkloadIdentity AuthMode = "workload_identity"
)
```

## oci.Config

```go
type Config struct {
	AuthMode AuthMode
	Region   string

	TenancyID   string
	UserID      string
	Fingerprint string
	PrivateKey  string // PEM content
	Passphrase  string // optional, for encrypted keys
}
```

Config holds OCI identity settings. The API-key fields are used only when
AuthMode is AuthAPIKey (or empty).

