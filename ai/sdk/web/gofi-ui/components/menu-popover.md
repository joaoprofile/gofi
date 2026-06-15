# Menu · Popover

Conteúdo flutuante ancorado a um trigger.

| Componente | Conteúdo | Uso |
|-----------|----------|-----|
| **Menu** (Dropdown) | uma lista de **ações**/opções | ações de linha, "mais", seleção rápida |
| **Popover** | conteúdo arbitrário | mini-form, detalhes, ajuda rica |

## Anatomia
Trigger → painel `--sf-card` + `--shadow-md` + `--radius-md` (`--z-dropdown`),
posicionado com tratamento de colisão (flip/shift) dentro do viewport.

## Estados
`closed · open · item hover/focus · item disabled`.

## Props
```ts
interface MenuItem { id: string; label: string; icon?: ReactNode; danger?: boolean;
  disabled?: boolean; onSelect: () => void; }
interface MenuProps { trigger: ReactNode; items: MenuItem[]; align?: 'start'|'end'; }
interface PopoverProps { trigger: ReactNode; children: ReactNode; }
```

## Acessibilidade
- Menu: `role="menu"`/`menuitem`; trigger `aria-haspopup` + `aria-expanded`.
  Teclado: setas navegam, Enter/Space ativa, Esc fecha, o foco **volta** ao
  trigger.
- Popover: `aria-expanded` no trigger; foco gerenciado; Esc/clique fora fecham.
- Item destrutivo: cor `--danger` **+** um label claro (não só cor).

## Do / Don't
- ✅ Agrupe e separe as ações destrutivas no final do menu.
- ❌ Um menu com dezenas de itens (mova para busca/seções). ❌ Um popover que é um
  modal disfarçado.

## Exemplo
```tsx
<Menu trigger={<IconButton aria-label="Mais ações"><DotsIcon/></IconButton>}
  items={[{id:'edit',label:'Editar',onSelect:edit},
          {id:'del',label:'Excluir',danger:true,onSelect:confirmDelete}]} />
```
