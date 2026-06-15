# Regras absolutas — mobile (React Native + TS)

Regras **inegociáveis** de código para a superfície mobile. Genéricas; o que é do
produto vive em `specs/` e `.claude/memory/`.

## Lib e estilo
1. **A lib `gofi-ui-native` é dependência npm — nunca a recrie.** Importe dela:
   `import { Button, Card, useTheme } from 'gofi-ui-native'`.
2. **Setup uma vez na raiz:** envolver a app em `<ThemeProvider brand={brand}>` (cores
   do projeto, de `ui.brand` no `.gofi.yaml`; omitido → padrão neutro).
   Componentes leem tokens via `useTheme()`; nada de tema/marca local.
3. **Estilo só via `StyleSheet.create` lendo `useTheme()`** — `backgroundColor:
   t.surfaceCard`, `padding: t.space[4]`. **Nunca** hex/medida solta. Em RN, `style`
   é o mecanismo normal (não é "inline CSS"); use `StyleSheet` para estilos estáticos.
4. **Um componente do DS antes de um novo** (variant, não clone).

## React Native específico
5. **Listas longas com `FlatList`/`SectionList`** (virtualizadas), nunca `.map()` num
   `ScrollView` para coleções grandes. Use o `ListItem`/`FeatureList` do DS na linha.
6. **Toque ≥ 44pt**; feedback via `Pressable` (estado `pressed`). Respeite
   **safe-area** (notch/home indicator) — ver `patterns/safe-area.md` (`Screen`).
7. **Sem API de web** (`window`, `document`, CSS). Navegação é **React Navigation**.

## TypeScript / estado / dados
8. **`strict` ligado; proibido `any`.** Não derive estado com `useEffect`.
9. **Sem `fetch` direto em componente** — camada de dados (hooks; TanStack Query é o
   padrão). 4 estados de servidor sempre: loading/empty/error/success.

## Acessibilidade (bloqueante)
10. `accessibilityRole` + `accessibilityLabel` em todo elemento interativo; suporte a
    leitor de tela (TalkBack/VoiceOver) e **Dynamic Type**. Contraste ≥ 4.5:1 (token).

## Testes
11. **React Native Testing Library** com queries acessíveis (`getByRole`,
    `getByLabelText`); `getByTestId` é último recurso.

> Mutação destrutiva exige confirmação ou undo. Animação respeita
> `AccessibilityInfo.isReduceMotionEnabled`.
</content>
