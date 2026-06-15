# Card

Uma superfície que agrupa conteúdo relacionado. Bloco de construção de dashboards e
listas (referência visual web).

## Anatomia
```
[ media? ]
[ header: título (h3) · ação? ]
[ body: conteúdo ]
[ footer: ações / metadados ]
```
Fundo `--sf-card`, borda `--sf-border`, raio `--radius-lg`,
`--shadow-sm`, padding `p-5`.

## Variantes
| Variante | Uso |
|---------|-----|
| `default` | conteúdo padrão sobre `--sf-card` |
| `brand` | superfície brand (`--brand` + `--tx-on-brand`) — hero/destaque |
| `interactive` | card clicável (hover eleva, focus visível) — envolver em `<a>`/`<button>` |
| `outlined` | só borda, sem sombra (alta densidade) |

## Estados (quando interactive)
`default · hover (--shadow-md) · focus (--focus) · disabled`.

## Props
```ts
interface CardProps {
  variant?: 'default' | 'brand' | 'interactive' | 'outlined';
  as?: 'div' | 'a' | 'article';
  header?: ReactNode; footer?: ReactNode; media?: ReactNode;
  href?: string;               // interactive
  children: ReactNode;
}
```

## Acessibilidade
- Um card totalmente clicável → use um `<a>`/`<button>` real envolvendo-o, não `onClick`
  num `<div>`. Evite "link dentro de link".
- `brand`: garanta contraste de texto (`--tx-on-brand`), nunca branco sobre a
  superfície de marca clara.

## Do / Don't
- ✅ Um título claro por card (h3) e uma ordem de leitura mobile coerente.
- ✅ Card de recomendação: tag de categoria + título + descrição + ação outline
  (ref. dashboard).
- ❌ Um card sem hierarquia (tudo do mesmo tamanho). ❌ Sombra pesada num card estático.

## Exemplo
```tsx
<Card variant="brand">
  <h3 style={{ color: 'var(--tx-on-brand)' }}>{title}</h3>
  <p style={{ color: 'var(--tx-on-brand)' }}>{subtitle}</p>
  <Button variant="primary">{cta}</Button>
</Card>
```
