# Empty State

O estado **vazio** é uma tela do produto, não um vácuo. Ele orienta e oferece a
próxima ação (princípio 12). Um dos 4 estados obrigatórios
([patterns/states.md](../patterns/states.md)).

## Anatomia
```
[ ilustração / ícone (decorativo) ]
[ título: o que está vazio, em tom neutro ]
[ descrição curta: por quê / o que fazer ]
[ CTA primário ] [ ação secundária? ]
```
Centralizado, com respiro generoso, sobre `--sf-card`/`--sf-page`.

## Tipos de vazio
| Tipo | Microcopy / ação |
|------|------------------|
| **Primeiro uso** (nunca teve dados) | convida a criar: "Crie seu primeiro {item}" + CTA |
| **Busca/filtro sem resultado** | "Nenhum resultado para «{termo}»" + limpar filtro |
| **Tudo resolvido** (inbox zero) | tom positivo: "Tudo em dia 🎉" |
| **Erro** (não é vazio) | use o estado de **erro** com retry, não o vazio |

## Props
```ts
interface EmptyStateProps { icon?: ReactNode; title: string; description?: string;
  action?: ReactNode; variant?: 'first-use' | 'no-results' | 'all-done'; }
```

## Acessibilidade
- Ilustração decorativa: `aria-hidden`. O título é o heading da região.
- O CTA é um botão/link real, alcançável pelo teclado.

## Do / Don't
- ✅ Distinguir "primeiro uso" de "busca sem resultado" (microcopy e ação diferentes).
  ✅ Tom acolhedor.
- ❌ Vazio sem ação ou explicação. ❌ Mostrar um erro como se fosse vazio.

## Exemplo
```tsx
<EmptyState variant="no-results" icon={<SearchIcon/>}
  title="Nenhum resultado para «{termo}»"
  description="Tente outro termo ou limpe os filtros."
  action={<Button variant="secondary" onClick={clear}>Limpar filtros</Button>} />
```
