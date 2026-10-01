# base/mail

`import "github.com/gofi-labs/gofi-sdk-go/base/mail"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package mail sends transactional and bulk e-mail over any SMTP provider, with
HTML + plain-text bodies, attachments and HTML templates. It is built to be
robust: TLS/STARTTLS, multiple SMTP AUTH mechanisms, timeouts, retry with
backoff, and connection reuse for bulk sends with per-message error
reporting. Build a Config explicitly and pass it to New; gofi's config
package can populate that Config from MAIL_* environment variables.

	m, err := mail.New(cfg)
	err = m.Send(ctx, &mail.Message{
	    From:    mail.Address{Name: "BlueFamly", Email: "no-reply@bluefamly.app"},
	    To:      []mail.Address{{Email: "user@example.com"}},
	    Subject: "Hello",
	    HTML:    "<h1>Oi</h1>",
	    Text:    "Oi",
	})

## Variáveis

```go
var (
	ErrNoRecipients  = errors.New("mail: message has no recipients")
	ErrNoSender      = errors.New("mail: message has no From address")
	ErrEmptyBody     = errors.New("mail: message has neither HTML nor text body")
	ErrNotConfigured = errors.New("mail: SMTP is not configured (MAIL_HOST/MAIL_FROM_EMAIL)")
	ErrInvalidConfig = errors.New("mail: invalid configuration")
	ErrInvalidHeader = errors.New("mail: address or header contains invalid characters")
)
```

Errors returned by the package.

```go
var ErrTemplateNotFound = errors.New("mail: template not found")
```

ErrTemplateNotFound is returned when rendering an unregistered template.

## mail.Address

```go
type Address struct {
	Name  string
	Email string
}
```

Address is an e-mail address with an optional display name.

### Address.String

```go
func (a Address) String() string
```

String renders the address as a header value, RFC 5322-encoding the name when
needed (e.g. `"João" <joao@example.com>`).

## mail.Attachment

```go
type Attachment struct {
	Filename    string
	ContentType string // defaults to application/octet-stream
	Content     []byte
}
```

Attachment is a file attached to a message.

## mail.AuthMechanism

```go
type AuthMechanism string
```

AuthMechanism is the SMTP authentication scheme.

```go
const (
	AuthNone    AuthMechanism = "none"
	AuthPlain   AuthMechanism = "plain"
	AuthLogin   AuthMechanism = "login"
	AuthCRAMMD5 AuthMechanism = "cram-md5"
)
```

## mail.BulkError

```go
type BulkError struct {
	Index int
	To    []string
	Err   error
}
```

BulkError describes the failure of a single message within a bulk send.

### BulkError.Error

```go
func (e BulkError) Error() string
```

### BulkError.Unwrap

```go
func (e BulkError) Unwrap() error
```

## mail.BulkResult

```go
type BulkResult struct {
	Sent   int
	Failed []BulkError
}
```

BulkResult reports the outcome of SendBulk: how many were delivered and which
ones failed (one bad recipient does not abort the whole batch).

### BulkResult.HasFailures

```go
func (r BulkResult) HasFailures() bool
```

HasFailures reports whether any message in the batch failed.

## mail.Config

```go
type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	From     Address

	Encryption Encryption
	Auth       AuthMechanism

	Timeout    time.Duration
	MaxRetries int // additional attempts after the first on transient failures
	PoolSize   int // bulk: reconnect every PoolSize messages (0 = single connection)
	HELODomain string

	// TLSConfig overrides the default tls.Config (ServerName = Host). Optional.
	TLSConfig *tls.Config
}
```

Config configures the SMTP mailer. Build it explicitly and pass it to New;
gofi's config package can populate it from MAIL_* environment variables.

## mail.Encryption

```go
type Encryption string
```

Encryption is the transport security used to reach the SMTP server.

```go
const (
	EncryptionNone     Encryption = "none"     // plaintext (dev only)
	EncryptionSTARTTLS Encryption = "starttls" // upgrade on 587/25
	EncryptionTLS      Encryption = "tls"      // implicit TLS on 465
)
```

## mail.Mailer

```go
type Mailer interface {
	// Send delivers a single message (with retry/backoff on transient failures).
	Send(ctx context.Context, msg *Message) error
	// SendBulk delivers many messages reusing the connection, capturing per-message
	// failures in the result. The returned error is non-nil only when the batch
	// could not start at all (e.g. cannot connect/authenticate).
	SendBulk(ctx context.Context, msgs []*Message) (BulkResult, error)
}
```

Mailer delivers messages. Implemented by the SMTP mailer; mockable in tests.

### mail.New

```go
func New(cfg Config) (Mailer, error)
```

New builds a Mailer from an explicit Config.

## mail.Message

```go
type Message struct {
	From        Address
	To          []Address
	Cc          []Address
	Bcc         []Address
	Subject     string
	HTML        string
	Text        string
	Attachments []Attachment
	Headers     map[string]string
}
```

Message is an e-mail to be delivered. At least one recipient, a From address
and one body (HTML or Text) are required. When both HTML and Text are set the
message is sent as multipart/alternative.

## mail.TemplateEngine

```go
type TemplateEngine struct {
	// contains filtered or unexported fields
}
```

TemplateEngine compiles and renders named e-mail templates. Safe for
concurrent use.

### mail.NewTemplateEngine

```go
func NewTemplateEngine() *TemplateEngine
```

NewTemplateEngine returns an empty engine.

### TemplateEngine.Has

```go
func (e *TemplateEngine) Has(name string) bool
```

Has reports whether a template is registered.

### TemplateEngine.Register

```go
func (e *TemplateEngine) Register(name string, src TemplateSource) error
```

Register compiles a template under name. Re-registering replaces it. Returns a
parse error if any source is malformed.

### TemplateEngine.Render

```go
func (e *TemplateEngine) Render(name string, data any) (subject, html, text string, err error)
```

Render executes the named template with data, returning subject, html and text.

### TemplateEngine.RenderMessage

```go
func (e *TemplateEngine) RenderMessage(name string, from Address, to []Address, data any) (*Message, error)
```

RenderMessage renders the named template into a ready-to-send Message addressed
from `from` to `to`.

## mail.TemplateSource

```go
type TemplateSource struct {
	Subject string
	HTML    string
	Text    string
}
```

TemplateSource holds the raw template strings for one named message. Subject
and Text use text/template; HTML uses html/template (auto-escaping). Any field
may be empty (e.g. HTML-only or Text-only messages).

