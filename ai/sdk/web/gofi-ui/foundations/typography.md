# Tipografia

Escala e pesos em [design-tokens.md](../../../../knowledge/ui/design-tokens.md).
No máximo **2 famílias tipográficas** (princípio 5).

## Escala e hierarquia

| Token | Uso | Regra |
|-------|-------|------|
| `--text-display` | título de marca / hero | 1 por tela, no máximo |
| `--text-h1` | título da página | exatamente 1 `<h1>` por página |
| `--text-h2` | seção | nunca pule um nível (h1 → h2 → h3) |
| `--text-h3` | título de card / subseção | — |
| `--text-body` | corpo padrão | legível no mínimo 16px |
| `--text-body-sm` | labels, apoio | — |
| `--text-caption` | legendas, badges | peso 500 para legibilidade |

## Regras

- **Hierarquia por tamanho/peso, não por cor.** Texto secundário usa
  `--tx-ink-2`, não um cinza avulso.
- **Redimensionar até 200%** sem quebrar (a11y): use `rem`/`clamp()`, nunca um `px`
  fixo que bloqueie o zoom. Ex: `font-size: clamp(1.5rem, 1.2rem + 1.5vw, 2.25rem)`.
- **Comprimento de linha** 45–75 caracteres no corpo de texto (`max-width: 65ch`).
- **Números tabulares** (`font-variant-numeric: tabular-nums`) em tabelas/valores
  para alinhar colunas.

```tsx
<h1 style={{ font: '700 var(--text-h1)/1.3 system-ui' }}>{title}</h1>
<p style={{ color: 'var(--tx-ink-2)', fontSize: 'var(--text-body-sm)' }}>
  {hint}
</p>
```

## Microcopy

Tom direto e claro. Use verbos de ação no idioma do produto ("Salvar", "Entrar",
"Sair", "Desfazer"); evite jargão. Detalhe em [patterns/forms.md](../patterns/forms.md).
