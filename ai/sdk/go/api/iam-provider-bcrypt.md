# iam/provider/bcrypt

`import "github.com/gofi-labs/gofi-sdk-go/iam/provider/bcrypt"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package bcrypt provides password hashing utilities for use in UserPort implementations.
Developers use this package in their implementation of port.UserPort.ValidatePassword.

## Constantes

```go
const (
	// DefaultCost is the default bcrypt cost (12). Adjust based on available hardware.
	// Costs below 12 are rejected in production — use MinCost only in tests.
	DefaultCost = 12
	MinCost     = bcrypt.MinCost
)
```

## Funções

### bcrypt.Compare

```go
func Compare(hash, password string) error
```

Compare checks whether the password matches the hash using timing-safe comparison.
Returns nil if the password is valid, or an error otherwise.
Never returns details that would allow distinguishing an invalid password from a corrupted hash.

### bcrypt.Hash

```go
func Hash(password string, cost int) (string, error)
```

Hash generates the bcrypt hash of a password with the given cost.
Returns an error if the cost is invalid or salt generation fails.

### bcrypt.HashDefault

```go
func HashDefault(password string) (string, error)
```

HashDefault generates the hash using DefaultCost (12).

