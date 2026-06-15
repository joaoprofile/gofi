# Field — mobile

Wrapper de controle: label + hint + erro. Igual em intenção ao
[web/field](../../../web/gofi-ui/components/field.md).

## Anatomia
```
Text(label)            [obrigatório?]
[ controle (Input/Select/...) ]
Text(hint | erro)   ← erro substitui hint quando inválido
```

## Props
```ts
interface FieldProps { label: string; hint?: string; error?: string;
  required?: boolean; children: ReactNode; }
```

## Acessibilidade
- Label como `Text` associado; controle recebe `accessibilityLabel` (o label) e,
  no erro, `accessibilityState={{ invalid: true }}` + a mensagem anunciada.
- Erro aparece após blur/submit, não no 1º caractere. Mensagem específica PT-BR.

## Do / Don't
- ✅ Label sempre visível (placeholder não substitui). ✅ Erro com texto, não só cor.
- ❌ Esconder o motivo do erro.
