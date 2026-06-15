# Estrutura de pastas — web (React)

Layout do app de UI sob `ui.path` (forma única) ou `ui.web.path` (multi-superfície).
A lib `gofi-ui` vem de `node_modules` — **não** há pasta de DS no projeto.

```
{ui.path}/src/
├── app/
│   ├── main.tsx            # entrypoint: import 'gofi-ui/styles' + render <Providers/>
│   ├── providers.tsx       # <ThemeProvider><QueryClientProvider> … </>
│   └── router.tsx          # rotas (lazy quando >100kb ou fora do caminho crítico)
├── pages/
│   └── {Page}.tsx          # composição de features + layout + <title>
├── features/
│   └── {contexto}/
│       ├── {Feature}.tsx           # UI + composição
│       ├── use{Feature}.ts         # estado local + queries/mutations
│       └── __tests__/{Feature}.test.tsx
├── components/             # reutilizáveis APP-SPECIFIC (não DS) — só se nenhum do DS serve
├── hooks/                  # hooks compartilhados entre features
└── lib/
    └── api/{contexto}.ts   # funções de I/O (fetch/axios) — consumidas por TanStack Query
```

## Convenções
- **Componentes/Pages:** `PascalCase` (`UserList.tsx`); **hooks:** `useCamelCase`;
  **api/utils:** `camelCase`.
- **Página = rota.** Uma página compõe features; não contém regra de I/O direta.
- **Feature = unidade de domínio na UI** — UI (`{Feature}.tsx`) + lógica
  (`use{Feature}.ts`). I/O sempre via `lib/api/{contexto}.ts` + TanStack Query.
- **DS vs app:** componente reutilizável e genérico já existe na `gofi-ui` (importe);
  `components/` do app é só para peça reutilizável **específica do produto**.
- **Imports:** DS de `'gofi-ui'`; gráficos de `'gofi-ui/charts'` quando aplicável.

> Multi-superfície: cada `ui.web`/`ui.mobile` tem o seu `path` e a sua árvore. Não
> compartilhe componentes de UI entre web e mobile (formas diferentes); compartilhe
> só tipos/contratos e lógica pura, se houver pacote comum.
</content>
