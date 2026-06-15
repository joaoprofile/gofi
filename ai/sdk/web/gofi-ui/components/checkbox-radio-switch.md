# Checkbox · Radio · Switch

Três controles booleanos com semânticas distintas — **não** são intercambiáveis.

| Controle | Significado | Cardinalidade | Efeito |
|---------|-------------|---------------|--------|
| **Checkbox** | seleção/aceite | 0..N independentes | aplicado ao confirmar |
| **Radio** | escolha exclusiva | 1 de N | aplicado ao confirmar |
| **Switch** | on/off | on/off | efeito **imediato** |

## Anatomia
Controle (raio `--radius-sm` para checkbox; pill para switch) + label clicável à
direita. O selecionado usa `--action`. Alvo de toque ≥ 44px (o label conta).

## Estados
`unchecked · checked · indeterminate (checkbox) · focus · disabled · invalid`.

## Props
```ts
interface ToggleProps {
  id: string;
  checked: boolean;            // (radio: agrupado por `name`)
  onChange: (checked: boolean) => void;
  label: string;
  indeterminate?: boolean;     // só checkbox (ex. "selecionar tudo" parcial)
  disabled?: boolean;
}
```

## Acessibilidade
- Inputs nativos (`type=checkbox|radio`, ou Switch com `role="switch"` +
  `aria-checked`). Label associado via `htmlFor`.
- Radios do mesmo grupo: mesmo `name`, dentro de `<fieldset>` + `<legend>`.
- Switch de efeito imediato: anuncie o resultado, não exija "salvar".

## Do / Don't
- ✅ Switch só quando o efeito é **imediato e reversível** (ex. notificações).
- ✅ Radio (não select) para 2–5 opções exclusivas visíveis.
- ❌ Switch para algo que só se aplica após "salvar" — use um checkbox.
- ❌ Clicar só na caixinha — o label inteiro é o alvo.

## Exemplo
```tsx
<Switch id="notif" checked={on} onChange={setOn} label="Receber notificações" />
<fieldset><legend>Visibilidade</legend>
  <Radio name="vis" id="pub" checked={v==='pub'} onChange={()=>setV('pub')} label="Público" />
  <Radio name="vis" id="prv" checked={v==='prv'} onChange={()=>setV('prv')} label="Privado" />
</fieldset>
```
