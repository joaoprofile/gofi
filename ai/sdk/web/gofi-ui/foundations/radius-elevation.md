# Radius e elevação

Valores em [design-tokens.md](../../../../knowledge/ui/design-tokens.md).
Geometria **arredondada e generosa**.

## Radius por componente

| Token | px | Componentes |
|-------|----|------------|
| `--radius-sm` | 8 | input, badge, tag, checkbox |
| `--radius-md` | 12 | botão (retangular), card pequeno, menu |
| `--radius-lg` | 16 | card, modal, drawer |
| `--radius-xl` | 24 | hero card, superfície de marca |
| `--radius-pill` | 999 | botão pill, chip, avatar, toggle |

> Botões de ação primária são **pill** por padrão (`--radius-pill`), como nas
> referências web e mobile. Cards e modais usam `lg`/`xl`.

## Elevação

Profundidade via **sombra suave**, não uma borda pesada. Quanto mais "flutuante" o
elemento, maior a sombra.

| Token | Uso |
|-------|-------|
| `--shadow-sm` | card em repouso, input em foco leve |
| `--shadow-md` | card elevado, dropdown, popover |
| `--shadow-lg` | modal, drawer, sheet |

- Card padrão: `--sf-card` + `--sf-border` (1px) + `--shadow-sm`.
- No **dark**, a sombra perde força → reforce a separação com `--sf-border`.
- Nunca empilhe uma sombra pesada em um elemento estático (poluição visual).
