# iam/provider/password

`import "github.com/gofi-labs/gofi-sdk-go/iam/provider/password"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package password hashes passwords with Argon2id (RFC 9106, OWASP's first
choice) in the PHC string format and verifies both Argon2id and bcrypt
hashes, so existing bcrypt users migrate on their next login:

	if err := password.Verify(stored, pw); err != nil { return errInvalid }
	if password.NeedsRehash(stored) {
	    newHash, _ := password.Hash(pw) // persist newHash
	}

## Variáveis

```go
var DefaultParams = Params{Memory: 19 * 1024, Time: 2, Threads: 1, SaltLen: 16, KeyLen: 32}
```

DefaultParams follow the OWASP baseline (19 MiB, 2 iterations, 1 lane):
strong while keeping concurrent logins affordable in small pods.

```go
var ErrInvalid = errors.New("iam/password: invalid credentials")
```

ErrInvalid is returned for a wrong password or an unreadable hash; the two
are deliberately indistinguishable.

## Funções

### password.Hash

```go
func Hash(password string) (string, error)
```

Hash returns the Argon2id PHC string of password with DefaultParams.

### password.NeedsRehash

```go
func NeedsRehash(hash string) bool
```

NeedsRehash reports whether hash should be replaced by a DefaultParams hash.

### password.Verify

```go
func Verify(hash, password string) error
```

Verify checks password against an Argon2id or bcrypt hash in constant time.

## password.Params

```go
type Params struct {
	Memory  uint32 // KiB
	Time    uint32 // iterations
	Threads uint8
	SaltLen uint32
	KeyLen  uint32
}
```

Params are the Argon2id cost parameters.

### Params.Hash

```go
func (p Params) Hash(password string) (string, error)
```

Hash returns the Argon2id PHC string of password with p.

### Params.NeedsRehash

```go
func (p Params) NeedsRehash(hash string) bool
```

NeedsRehash is true for bcrypt, unreadable hashes and Argon2id hashes
weaker than p.

