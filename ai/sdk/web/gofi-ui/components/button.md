# Button

Ação do usuário. O primary é um **pill** (`--radius-pill`) preenchido com `--action`
— como nas referências web e mobile.

## Anatomia
`[ ícone? · label · ícone? ]` — padding `p-3`/`p-5`, altura mínima 44px.

## Variantes
| Variante | Fundo | Texto | Uso |
|---------|-------|-------|-----|
| `primary` | `--action` | `#fff` | ação principal da tela (1 por contexto) |
| `secondary` | transparente + borda `--action` | `--action` | ação alternativa |
| `ghost` | transparente | `--tx-ink` | ação terciária, em toolbars |
| `danger` | `--danger` | `#fff` | ação destrutiva |
| `brand` | `--brand` | `--tx-on-brand` | CTA sobre superfície clara/hero |

Tamanhos: `sm` (32) · `md` (40) · `lg` (48). Largura: `auto` ou `full` (mobile).

## Estados
`default · hover (--action-hover) · active/pressed · focus (--focus) ·
disabled (opacidade 40%, sem ponteiro) · loading (spinner + aria-busy, label mantido)`.

## Props
```ts
interface ButtonProps {
  variant?: 'primary' | 'secondary' | 'ghost' | 'danger' | 'brand'; // default primary
  size?: 'sm' | 'md' | 'lg';        // default md
  full?: boolean;                    // largura total (comum em mobile/forms)
  loading?: boolean;                 // mostra spinner, desabilita, aria-busy
  iconStart?: ReactNode; iconEnd?: ReactNode;
  type?: 'button' | 'submit';        // default button
  disabled?: boolean;
  onClick?: () => void;
  children: ReactNode;               // label (verbo de ação: "Salvar", "Entrar")
}
```

## Acessibilidade
- Elemento `<button>` nativo (nunca `<div onClick>`).
- `loading` → `aria-busy="true"`, mantém o label (não troque por "carregando…").
- Focus visível via `--focus`. Alvo ≥ 44px.
- **Icon Button**: um `aria-label` com a ação é obrigatório.

## Do / Don't
- ✅ Verbo de ação no label: "Salvar", "Entrar", "Adicionar".
- ✅ Um único `primary` por contexto visual.
- ❌ Texto branco sobre `brand` (superfície de marca clara) — use `--tx-on-brand`.
- ❌ Desabilitar sem explicar por quê (prefira validar e mostrar um erro).

## Exemplo
```tsx
<Button variant="primary" full loading={isSaving} onClick={save}>Salvar</Button>
<Button variant="secondary" iconStart={<PlusIcon />}>Adicionar</Button>
<IconButton aria-label="Fechar" onClick={close}><CloseIcon /></IconButton>
```
