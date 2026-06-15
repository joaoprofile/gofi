# Layout (primitivos) — mobile

Primitivos de composição que aplicam a **escala 4/8** sobre `View`/flex, sem medida
solta. Mais enxuto que o [web](../../../web/gofi-ui/components/layout.md): `Stack`,
`Row`, `Divider`.

## Variantes
| Componente | Faz |
|-----------|------|
| `Stack` | coluna com `gap` consistente (`flexDirection: column`) |
| `Row` | linha com `gap` (`extends StackProps` + `wrap`) |
| `Divider` | separador usando `surfaceBorder` |

## Props (TS)
```ts
interface StackProps {
  gap?: SpaceKey;                 // token 4/8 (default 4)
  align?: FlexAlign;              // alignItems
  justify?: FlexJustify;          // justifyContent
  style?: StyleProp<ViewStyle>;
  children: ReactNode;
}
interface RowProps extends StackProps { wrap?: boolean }   // default gap 2, align 'center'
interface DividerProps { style?: StyleProp<ViewStyle> }
```
`gap` indexa `base.space[gap]` (4/8). Nada de número mágico.

## a11y
Layout não altera a ordem de leitura (a ordem dos filhos = ordem do leitor de tela).
`Divider` é decorativo.

## Do / Don't
- ✅ `Stack`/`Row` para espaçamento por token; ✅ `gap` da escala.
- ❌ `style={{ marginTop: 13 }}`; ❌ aninhar `ScrollView` em lista longa (use `FlatList`).

## Exemplo
```tsx
import { Stack, Row, Divider } from 'gofi-ui-native';

<Stack gap={5}>
  <Row justify="space-between"><Text variant="h2">Título</Text><Button>Novo</Button></Row>
  <Divider />
  <Stack gap={3}>{itens}</Stack>
</Stack>
```
</content>
