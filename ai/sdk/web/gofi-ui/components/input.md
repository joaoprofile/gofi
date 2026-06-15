# Input / Textarea

Entrada de texto. Sempre dentro de um [Field](field.md) (label + erro + hint).

## Anatomia
`[ ícone? · texto digitado · ação? (limpar/olho) ]` — altura 44px, raio `--radius-sm`,
borda `--sf-border`, fundo `--sf-card`.

## Variantes
- Por tipo: `text · email · password · number · search · tel · url`.
- `Textarea`: multilinha, `min-height` 96px, auto-resize opcional.
- Afixos: ícone/elemento à esquerda (busca) ou à direita (limpar, mostrar senha, unidade).

## Estados
| Estado | Visual |
|-------|--------|
| default | borda `--sf-border` |
| focus | borda `--action` + `--focus` |
| invalid | borda `--danger` (+ mensagem no Field) |
| disabled | fundo `--sf-hover`, texto secundário |
| readonly | sem borda editável, texto normal |

## Props
```ts
interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  id: string;                  // liga ao Field
  invalid?: boolean;           // → aria-invalid
  iconStart?: ReactNode; iconEnd?: ReactNode;
}
```

## Acessibilidade
- `id` ligado ao `<label htmlFor>`. `aria-invalid` quando inválido.
- O `type` certo aciona o teclado mobile e o autofill certos (`email`, `tel`).
- `autoComplete` apropriado. Senha: um botão "mostrar/ocultar" com `aria-label`.
- Autofill no dark-mode: ver [theming-dark-mode.md](../../../../knowledge/ui/theming-dark-mode.md).

## Do / Don't
- ✅ Máscara/formatação só na exibição; valor cru no state.
- ✅ Busca com ícone à esquerda e botão "limpar" à direita (ref. dashboard).
- ❌ Validar a cada tecla (valide no blur/submit). ❌ Placeholder como label.

## Exemplo
```tsx
<Field label="Buscar" htmlFor="q">
  <Input id="q" type="search" iconStart={<SearchIcon />} placeholder="Buscar…" />
</Field>
```
