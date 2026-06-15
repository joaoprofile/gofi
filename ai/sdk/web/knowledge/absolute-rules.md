# Regras absolutas — web (React + TS + Tailwind v4)

Regras **inegociáveis** de código para a superfície web. Genéricas (domínio-neutro);
o que é do produto vive em `specs/` e `.claude/memory/`.

## Lib e estilo
1. **A lib `gofi-ui` é dependência npm — nunca a recrie.** Importe componentes e
   tipos dela: `import { Button, Card, type ButtonProps } from 'gofi-ui'`. Os docs
   do DS são a especificação do que ela expõe.
2. **Setup uma vez na raiz:** `import 'gofi-ui/styles'` + envolver a app em
   `<ThemeProvider>` (define `data-theme` e injeta as **cores do projeto** — do
   bloco `ui.brand` do `.gofi.yaml` — como vars `--brand`/`--action`/etc.; a lib
   aceita cores arbitrárias). Nenhum componente gerencia tema/marca localmente.
3. **Estilo só por utilitário Tailwind** (tokens do DS): `className="bg-card text-ink
   rounded-lg"`. **Nunca** literal de cor/medida; **nunca** `style={}` salvo valor
   computado em runtime (ex.: posição de tooltip).
4. **Um componente do DS antes de um novo.** Precisa de variação → uma *variant*,
   não um clone. Componente app-specific só quando nenhum do DS resolve.

## TypeScript
5. **`strict` ligado; proibido `any`** (use `unknown` + narrowing, generics, ou o
   tipo exportado pela lib/contrato).
6. **Não derive estado com `useEffect`.** Valor que vem de props/estado é calculado
   no render (ou `useMemo`); `useEffect` é só para efeito colateral real (I/O,
   subscription, sincronizar com algo externo).
7. **Sem prop drilling > 2 níveis** — extraia hook/contexto/serviço.

## Dados
8. **Sem `fetch` direto em componente.** Toda I/O passa por **TanStack Query**
   (`useQuery`/`useMutation`) sobre uma função da camada `lib/api/{contexto}.ts`.
9. **Estados de servidor são 4:** loading, empty, error, success — tratados na
   feature/página (ver `patterns/states.md`). Nunca só o caminho feliz.

## Acessibilidade (bloqueante)
10. Todo input tem **label associado** (placeholder não conta), erro e hint quando
    aplicável. Foco **sempre visível** (nunca `outline:none` sem alternativa).
    Navegação por teclado funcional. `aria-*` quando o HTML semântico não cobre.
11. Contraste ≥ 4.5:1 (texto) / 3:1 (UI) — via token, verificado com ferramenta.

## Testes
12. **Queries acessíveis** (`getByRole`, `getByLabelText`) — `getByTestId` é último
    recurso. I/O mockado por handcraft; MSW só se a spec exigir.

> Mutação destrutiva exige **confirmação ou undo** (nunca ambos ausentes).
> Animação > 200ms respeita `prefers-reduced-motion`.
</content>
