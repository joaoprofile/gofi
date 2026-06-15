# Layout (primitivos)

Primitivos de composição que aplicam a **escala 4/8** e o flex/grid sem CSS solto.
Substituem `style` de layout por uma API de tokens. Também exportam o `Divider`.

## Variantes
| Componente | Faz |
|-----------|------|
| `Stack` | empilha na vertical com `gap` consistente |
| `Inline` | linha horizontal com `gap` + `wrap` |
| `Grid` | grid responsivo (`min` por coluna, ou `cols` fixo) |
| `Container` | largura máxima + centralização (`sm`…`full`) |
| `Divider` | separador semântico usando `--sf-border` |

## Props (TS)
```ts
interface StackProps   extends HTMLAttributes<HTMLDivElement> { gap?: Gap; align?: …; justify?: …; as?: ElementType; }
interface InlineProps  extends HTMLAttributes<HTMLDivElement> { gap?: Gap; align?: …; justify?: …; wrap?: boolean; as?: ElementType; }
interface GridProps    extends HTMLAttributes<HTMLDivElement> { min?: string; cols?: number; gap?: Gap; }
interface ContainerProps extends HTMLAttributes<HTMLDivElement> { size?: 'sm'|'md'|'lg'|'xl'|'full'; }
interface DividerProps extends HTMLAttributes<HTMLHRElement>   { orientation?: 'horizontal'|'vertical'; }
```
`gap` vem da escala 4/8 (mesma de `p-4`/`gap-6`).

## a11y
`Divider` decorativo → sem papel; separando grupos de navegação → use
`role="separator"`. Layout não rouba ordem de leitura (DOM = ordem visual).

## Do / Don't
- ✅ `Stack`/`Inline`/`Grid` para espaçamento; ✅ `Container` para largura de página.
- ❌ `style={{ display:'flex', gap:12 }}` solto; ❌ `gap` fora da escala.

## Exemplo
```tsx
import { Container, Stack, Inline, Grid, Divider } from 'gofi-ui';

<Container size="lg">
  <Stack gap={6}>
    <Inline gap={2} justify="between"><h1 className="text-h1 text-ink">Título</h1><Button>Novo</Button></Inline>
    <Divider />
    <Grid min="240px" gap={4}>{cards}</Grid>
  </Stack>
</Container>
```
</content>
