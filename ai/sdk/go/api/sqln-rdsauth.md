# sqln/rdsauth

`import "github.com/joaoprofile/gofi-sdk-go/sqln/rdsauth"` · SDK gofi/v0.2.4 (310ba1a)

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package rdsauth signs RDS / Aurora IAM authentication tokens, so databases
accept the pod's AWS identity instead of a static password:

	awsCfg, _ := cloudaws.Load(ctx, cloudaws.Config{})
	cfg.Password = rdsauth.Password(awsCfg, "db.xxxx.rds.amazonaws.com:5432", "app")

Tokens last 15 minutes and are signed for every new connection. A token is
a bearer credential, so the postgres driver refuses Config.Password unless
the DSN verifies the server: sslmode=verify-full (or verify-ca) with the
RDS CA bundle as sslrootcert.

## Funções

### rdsauth.Password

```go
func Password(awsCfg awssdk.Config, endpoint, user string) func(ctx context.Context) (string, error)
```

Password returns a callback for sqln's connection.Config.Password.
endpoint is host:port of the instance, cluster or proxy.

