# sqln/transaction

`import "github.com/gofi-labs/gofi-sdk-go/sqln/transaction"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## transaction.Options

```go
type Options struct {
	Isolation sql.IsolationLevel
	ReadOnly  bool
	// Connection selects the database; nil uses the global connection.
	Connection *connection.Connection
	// MaxRetries re-runs fn after serialization failures and deadlocks
	// (connection.IsRetryable). fn must be safe to repeat: no side effects
	// outside the transaction.
	MaxRetries int
}
```

Options configures a transaction.

## transaction.Transaction

```go
type Transaction interface {
	Execute(ctx context.Context, fn func(ctx context.Context) error) error
}
```

### transaction.New

```go
func New(opts Options) Transaction
```

New returns a Transaction with opts.

### transaction.NewTransaction

```go
func NewTransaction(isolation ...sql.IsolationLevel) Transaction
```

NewTransaction returns a Transaction with the given isolation level.

