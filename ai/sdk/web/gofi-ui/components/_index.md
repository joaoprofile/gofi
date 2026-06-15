# Catálogo de componentes — Web

Taxonomia atômica. **Componha os existentes antes de criar um novo**; uma nova
variação é uma *variante*, não um clone. Todos os imports vêm de `gofi-ui`. Cada arquivo
segue o mesmo template:

> **anatomia · variantes · estados · props (TS) · a11y · do/don't · exemplo**

> **Cobertura.** Este catálogo mapeia **todos** os componentes que a lib `gofi-ui`
> exporta. `Arquivo = — (criar)` marca o que a lib já entrega mas ainda não tem doc.
> **Nome de export quando difere do arquivo:** o booleano (checkbox/radio/switch)
> é exportado como **`Toggle`**. A cor de apoio na lib web é o token **`accent`**
> (`bg-accent`) — o mobile mantém `secondary`; fonte única em
> [design-tokens.md](../../../../knowledge/ui/design-tokens.md).

## Átomos (primitivos)
| Componente | Arquivo | Resumo |
|-----------|------|--------|
| Button | [button.md](button.md) | ação; pill; variantes primary/secondary/ghost/danger |
| Icon Button | [button.md](button.md) | ação só com ícone (aria-label obrigatório) |
| Badge / Tag / Chip | [badge-tag.md](badge-tag.md) | rótulo de status/categoria; chip removível |
| Avatar | [avatar.md](avatar.md) | imagem/iniciais; **stack +N** |
| Spinner / Skeleton | [skeleton-spinner.md](skeleton-spinner.md) | loading contextual |
| Progress | [progress.md](progress.md) | barra/círculo de progresso |
| Tooltip | [tooltip.md](tooltip.md) | dica curta no hover/focus |
| Divider | [layout.md](layout.md) | `<hr>` com `--sf-border` (exportado pelo Layout) |

## Formulários
| Componente | Arquivo | Resumo |
|-----------|------|--------|
| Field | [field.md](field.md) | wrapper de label + hint + erro (base de todo input) |
| Input / Textarea | [input.md](input.md) | texto, com estados de validação |
| Select / Combobox | [select.md](select.md) | seleção única/busca |
| Checkbox / Radio / Switch | [checkbox-radio-switch.md](checkbox-radio-switch.md) | booleanos e escolha (export `Toggle`) |
| Segmented Control | [segmented-control.md](segmented-control.md) | troca de visão (ref. dashboard) |
| Date Picker | [datepicker.md](datepicker.md) | data/intervalo/hora; campo + calendário em popover |

## Contêineres e dados
| Componente | Arquivo | Resumo |
|-----------|------|--------|
| Card | [card.md](card.md) | superfície de agrupamento; variante brand |
| List / List Item | [list.md](list.md) | lista vertical, item com avatar/ação |
| Table / DataTable | [table.md](table.md) | tabela rica (ref. dashboard) |
| Tabs | [tabs.md](tabs.md) | navegação entre painéis |
| Accordion | [accordion.md](accordion.md) | seções expansíveis |
| Stepper | [stepper.md](stepper.md) | fluxo multi-etapa |
| Pagination | [pagination.md](pagination.md) | navegação por páginas (ref. dashboard) |
| Empty State | [empty-state.md](empty-state.md) | vazio com ilustração + CTA |

## Overlay e feedback
| Componente | Arquivo | Resumo |
|-----------|------|--------|
| Modal / Drawer | [modal-drawer.md](modal-drawer.md) | diálogo centralizado / painel lateral |
| Toast / Banner | [toast-banner.md](toast-banner.md) | notificação transitória / persistente |
| Menu / Popover | [menu-popover.md](menu-popover.md) | ações contextuais / conteúdo flutuante |
| Confirm Dialog | [modal-drawer.md](modal-drawer.md) | confirmar uma ação destrutiva |

## Layout e gráficos
| Componente | Arquivo | Resumo |
|-----------|------|--------|
| Layout (primitivos) | [layout.md](layout.md) | Stack / Inline / Grid / Container / Divider |
| Charts | [charts.md](charts.md) | Area / Bar / Line / Donut + ChartContainer; subpath `gofi-ui/charts` |

> **Estrutura do projeto:** componentes reutilizáveis ficam em `components/` da app;
> os específicos de feature em `features/{contexto}/`. Ver
> [skills/gofi-ui.md](../../../../skills/gofi-ui.md) §Workflow.
