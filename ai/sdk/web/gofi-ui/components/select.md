# Select / Combobox

Escolha uma opção entre várias. `Select` = lista fixa; `Combobox` = com
busca/filtro. Dentro de um [Field](field.md).

## Quando usar
| Situação | Componente |
|----------|-----------|
| ≤ 5 opções exclusivas | Radio ([checkbox-radio-switch](checkbox-radio-switch.md)) |
| 5–15 opções | Select |
| > 15 opções ou busca | Combobox |
| escolha múltipla | Combobox multi ou checkboxes |

## Anatomia
`[ valor selecionado / placeholder · chevron ]` → painel `--sf-card` +
`--shadow-md`, itens com hover `--sf-hover`, selecionado com um check + `--action`.

## Estados
`default · focus · open · invalid · disabled · loading (opções async) · empty
(sem opções / sem resultado de busca)`.

## Props
```ts
interface SelectProps<T> {
  id: string;
  value: T | null;
  options: Array<{ value: T; label: string; disabled?: boolean }>;
  onChange: (v: T) => void;
  searchable?: boolean;        // vira um Combobox
  multiple?: boolean;
  placeholder?: string;        // ex. "Selecione…"
  invalid?: boolean;
  loading?: boolean;
}
```

## Acessibilidade
- Padrão ARIA combobox/listbox: `role="combobox"` + `aria-expanded` +
  `aria-controls`; lista `role="listbox"`, item `role="option"` + `aria-selected`.
- Teclado: setas navegam, Enter seleciona, Esc fecha, digitar filtra
  (`aria-activedescendant`).
- O foco volta ao trigger ao fechar.

## Do / Don't
- ✅ Estado **empty** no Combobox: "Nenhum resultado para «{termo}»".
- ✅ Agrupe listas longas de opções com um cabeçalho.
- ❌ Um select nativo estilizado de forma inacessível — prefira o padrão ARIA completo
  ou o `<select>` nativo de verdade.

## Exemplo
```tsx
<Field label="Status" htmlFor="status">
  <Select id="status" value={status} onChange={setStatus}
          options={[{value:'a',label:'Ativo'},{value:'i',label:'Inativo'}]} />
</Field>
```
