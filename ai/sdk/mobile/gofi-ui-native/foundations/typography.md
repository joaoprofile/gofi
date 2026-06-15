# Tipografia — mobile

Escala em [design-tokens.md](../../../../knowledge/ui/design-tokens.md). Componente
`Text` do DS encapsula os papéis (ver [components/text.md](../components/text.md)).

## Dynamic Type (acessibilidade)
- Respeite o tamanho de fonte do sistema: **não** desligue `allowFontScaling`
  (default true). Teste com fonte grande do SO.
- Limite o fator máximo só onde quebra layout: `maxFontSizeMultiplier` (ex.: 1.6),
  nunca `allowFontScaling={false}`.
- Layouts crescem com o texto (sem altura fixa que corte).

## Papéis (mesma escala do web)
`display · h1 · h2 · h3 · body · body-sm · caption`. Máx. 2 typefaces.

## Regras
- Hierarquia por tamanho/peso, não por cor; secundário = `textSecondary`.
- Números tabulares para valores/listas alinhadas.
- `numberOfLines` + `ellipsizeMode` para truncar com segurança.

```tsx
<Text variant="h1">{title}</Text>
<Text variant="bodySm" color="secondary" maxFontSizeMultiplier={1.6}>{hint}</Text>
```
