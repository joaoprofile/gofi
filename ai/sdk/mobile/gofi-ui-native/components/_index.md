# Catálogo de componentes — Mobile (RN)

Espelha o web ([web/components/_index.md](../../../web/gofi-ui/components/_index.md))
com primitivos React Native. Mesmo template: **anatomia · variantes · estados ·
props · a11y · do/don't · exemplo**. Componha antes de criar; variant, não clone.

> **Cobertura.** Mapeia **todos** os componentes que a lib `gofi-ui-native`
> exporta. `Arquivo = — (criar)` marca o que a lib entrega sem doc ainda.
> **Nomes de export quando diferem do arquivo:** "Modal" é exportado como
> **`Dialog`**; "Safe Area" como **`Screen`**; o booleano (checkbox/radio/switch)
> como **`Toggle`**. A cor de apoio no mobile mantém o token **`secondary`** (o web
> usa `accent`).

## Átomos
| Componente | Arquivo | Resumo |
|------------|---------|--------|
| Text | [text.md](text.md) | tipografia por variante + Dynamic Type |
| Button | [button.md](button.md) | `Pressable` pílula; variantes |
| Badge / Chip | [badge-chip.md](badge-chip.md) | status/categoria/filtro |
| Avatar | [avatar.md](avatar.md) | imagem/iniciais + stack +N |
| Skeleton / Spinner | [skeleton.md](skeleton.md) | loading |
| Progress | [progress.md](progress.md) | barra de progresso linear |

## Formulário
| Componente | Arquivo | Resumo |
|------------|---------|--------|
| Field | [field.md](field.md) | label + hint + erro |
| Input | [input.md](input.md) | `TextInput` com estados |
| Checkbox / Radio / Switch | [checkbox-radio-switch.md](checkbox-radio-switch.md) | booleanos (export `Toggle`) |
| Segmented Control | [segmented-control.md](segmented-control.md) | troca de visão/segmento (tipado) |

## Contêineres e dados
| Componente | Arquivo | Resumo |
|------------|---------|--------|
| Card | [card.md](card.md) | superfície; variante de marca |
| List Item | [list-item.md](list-item.md) | linha rica (use em `FlatList`) |
| Feature List | [feature-list.md](feature-list.md) | linhas ícone+texto (ref. mockup) |
| Empty State | [empty-state.md](empty-state.md) | vazio com ilustração + CTA |

## Navegação e overlay
| Componente | Arquivo | Resumo |
|------------|---------|--------|
| Header | [header.md](header.md) | cabeçalho de tela (título + voltar + ações) |
| Tab Bar | [tab-bar.md](tab-bar.md) | navegação inferior (3–5 destinos) |
| Bottom Sheet | [bottom-sheet.md](bottom-sheet.md) | painel deslizante inferior |
| Modal | [modal.md](modal.md) | diálogo/confirmação (export `Dialog`) |
| Toast | [toast.md](toast.md) | feedback transitório |
| Safe Area | [../patterns/safe-area.md](../patterns/safe-area.md) | bordas seguras (export `Screen`) |

## Layout e gráficos
| Componente | Arquivo | Resumo |
|------------|---------|--------|
| Layout (primitivos) | [layout.md](layout.md) | Stack / Row / Divider |
| Charts | [charts.md](charts.md) | BarChart / DonutChart / Sparkline (tokens) |
