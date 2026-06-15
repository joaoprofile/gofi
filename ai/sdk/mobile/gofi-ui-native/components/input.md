# Input — mobile

`TextInput` dentro de um [Field](field.md). Altura ≥ 44pt, raio `sm`, borda
`surfaceBorder`, fundo `surfaceCard`.

## Estados
`default · focus (borda colorAction) · invalid (borda danger) · disabled · readonly`.

## Props
```ts
interface InputProps extends RNTextInputProps {
  invalid?: boolean; iconStart?: ReactNode; iconEnd?: ReactNode;
}
```

## RN específico
- `keyboardType` correto (`email-address`, `numeric`, `phone-pad`), `autoCapitalize`,
  `autoComplete`/`textContentType` (autofill/iOS), `secureTextEntry` (senha + botão olho).
- `returnKeyType` e `onSubmitEditing` para encadear campos; `KeyboardAvoidingView`
  no formulário.
- Busca: ícone à esquerda + limpar à direita.

## Acessibilidade
- `accessibilityLabel` = label do Field; `accessibilityState={{ invalid }}`.
- Não desligue Dynamic Type. Senha com botão "mostrar/ocultar" rotulado.

## Exemplo
```tsx
<Field label="E-mail" error={err}>
  <Input keyboardType="email-address" autoCapitalize="none"
         textContentType="emailAddress" invalid={!!err} />
</Field>
```
