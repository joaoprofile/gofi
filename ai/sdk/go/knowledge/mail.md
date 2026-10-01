---
name: mail
description: E-mail com gofi base/mail — Mailer aberto no composition root, templates, bulk com falha por mensagem, degradação quando SMTP não está configurado
sdk: v0.8.2
keywords: [mail, email, smtp, template, bulk, notification, MAIL_HOST]
---

# E-mail — `base/mail`

Referência gerada: `.claude/sdk/go/api/base-mail.md`; env → config em
`gofi-config.md` § `config.Mail` / `config.NewMailer`.

Import: `github.com/gofi-labs/gofi-sdk-go/base/mail`. SMTP puro (qualquer
provedor): HTML + texto, anexos, templates, TLS/STARTTLS, retry com backoff,
reuso de conexão em bulk.

---

## Composition root — abrir uma vez, tolerar "não configurado"

```go
import (
    "errors"

    "github.com/gofi-labs/gofi-sdk-go/base/mail"
    "github.com/gofi-labs/gofi-sdk-go/gofi/config"
)

func buildMailer(env *environment.Environment) mail.Mailer {
    m, err := config.NewMailer(env) // MAIL_* → mail.Config → mail.New
    if errors.Is(err, mail.ErrNotConfigured) {
        logging.Warn("mail not configured; notifications disabled")
        return nil
    }
    if err != nil {
        logging.Fatal("mail: invalid configuration", slog.Any("error", err))
    }
    return m
}
```

- `ErrNotConfigured` = faltou `MAIL_HOST` ou `MAIL_FROM_EMAIL` → feature off
  (nil + nil-check no wrapper). Config **presente mas inválida** é erro de
  deploy → falha o boot.
- Sem env do gofi: `mail.New(mail.Config{Host, Port, Username, Password, From, Encryption, Auth, ...})`.

Env: `MAIL_HOST`, `MAIL_PORT`, `MAIL_USERNAME`, `MAIL_PASSWORD`,
`MAIL_FROM_NAME`, `MAIL_FROM_EMAIL`, `MAIL_ENCRYPTION` (`none|starttls|tls`),
`MAIL_AUTH` (`plain|login|cram-md5|none`), `MAIL_TIMEOUT`, `MAIL_MAX_RETRIES`,
`MAIL_POOL_SIZE`, `MAIL_HELO_DOMAIN`. Senha via `secret://…` ou `MAIL_PASSWORD_FILE`.

---

## Envio — atrás de um wrapper de domínio

O domínio depende de uma interface **do contexto** (ex.: `{Ctx}Notifier`), não de
`mail.Mailer` espalhado; o wrapper faz nil-check, escolhe template e traduz erro
para `errs.AppError`.

```go
engine := mail.NewTemplateEngine() // uma vez no boot; seguro para concorrência
if err := engine.Register("{template}", mail.TemplateSource{
    Subject: "{{.Title}}",
    HTML:    htmlTpl, // html/template (auto-escape)
    Text:    textTpl, // text/template
}); err != nil {
    return err // template malformado falha no boot, não no envio
}

msg, err := engine.RenderMessage("{template}", from, []mail.Address{{Name: n, Email: e}}, data)
if err != nil { return err }
err = mailer.Send(ctx, msg)
```

- `Message` exige remetente, ≥1 destinatário e HTML ou Text (os dois → multipart);
  erros `ErrNoSender`, `ErrNoRecipients`, `ErrEmptyBody`, `ErrInvalidHeader`.
- `Send` já faz retry com backoff em falha transitória (`MaxRetries`) — não
  envolver em retry próprio.
- **Bulk:** `res, err := mailer.SendBulk(ctx, msgs)` — `err` só quando o lote nem
  começa (conexão/auth); falhas por mensagem ficam em `res.Failed`
  (`[]mail.BulkError{Index, To, Err}`), checar `res.HasFailures()`.
- Envio em volume ou que não pode bloquear o request → publicar evento e enviar
  num consumer (`messaging-msq.md`), não no handler HTTP.

---

## Testes

`mail.Mailer` é interface: fake handcraft que grava as `*mail.Message`
recebidas. Templates testam-se com `engine.Render(name, data)` (devolve
subject, html, text) — sem SMTP.

## Anti-padrões

- ❌ `net/smtp` direto ou outro cliente SMTP no projeto.
- ❌ Montar HTML por concatenação de string com dado do usuário — use
  `TemplateSource.HTML` (auto-escape).
- ❌ Registrar template no caminho do envio (compila a cada chamada; erro tardio).
- ❌ Ignorar `BulkResult.Failed` — `err == nil` não significa todos entregues.
- ❌ Fatalizar boot por `ErrNotConfigured` quando e-mail é feature opcional.
