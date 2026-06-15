# Estrutura de pastas — mobile (React Native)

Layout do app sob `ui.path` (forma única) ou `ui.mobile.path` (multi-superfície). A
lib `gofi-ui-native` vem de `node_modules` — **não** há pasta de DS no projeto.

```
{ui.path}/src/
├── app/
│   ├── App.tsx             # raiz: <ThemeProvider><NavigationContainer> … </>
│   └── providers.tsx       # ThemeProvider + QueryClientProvider (se houver)
├── navigation/
│   ├── RootNavigator.tsx   # stack raiz
│   └── TabNavigator.tsx    # tabs (usa o TabBar do DS / React Navigation)
├── screens/
│   └── {Screen}.tsx        # tela = destino de navegação (compõe features)
├── features/
│   └── {contexto}/
│       ├── <Feature>.tsx
│       ├── use<Feature>.ts
│       └── __tests__/<Feature>.test.tsx
├── components/             # reutilizáveis APP-SPECIFIC (não DS)
├── hooks/
└── lib/
    └── api/{contexto}.ts   # I/O
```

## Convenções
- **Screens/components:** `PascalCase`; **hooks:** `useCamelCase`.
- **Screen = destino de navegação** (registrado no navigator); compõe features.
- **Feature = unidade de domínio** (UI + `use<Feature>`); I/O via `lib/api`.
- **Navegação:** React Navigation (stack/tab); a `TabBar`/`Header` do DS são UI, o
  roteamento é do React Navigation.
- **Imports:** DS de `'gofi-ui-native'`; envolva telas no `Screen` (safe-area) do DS.

> Multi-superfície: web e mobile têm árvores separadas e **formas diferentes** — não
> compartilhe componentes de UI; compartilhe só tipos/contratos e lógica pura.
</content>
