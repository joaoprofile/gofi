# Pattern — Formulários mobile

Mesmos princípios do [web](../../../web/gofi-ui/patterns/forms.md), com o
essencial de teclado/toque do RN.

## Estrutura
- Uma coluna; campos via [Field](../components/field.md) + [Input](../components/input.md).
- `KeyboardAvoidingView` + `ScrollView` (`keyboardShouldPersistTaps="handled"`) para
  o teclado não cobrir o campo ativo.
- Encadear campos: `returnKeyType="next"` + `onSubmitEditing` move ao próximo; último
  é `done` e submete.
- Ação principal (salvar) em botão pílula `full`, fixo acima do teclado/rodapé
  (respeita safe area).

## Validação
- No blur/submit (não no 1º caractere). Foco no primeiro inválido ao submeter.
- Erro = `accessibilityState={{ invalid }}` + texto específico PT-BR.

## Teclado e tipos
- `keyboardType`/`textContentType`/`autoComplete` corretos (acelera + autofill).
- `secureTextEntry` com botão mostrar/ocultar rotulado.

## Microcopy
"Salvar", "Entrar", "Sair", "Adicionar", "Desfazer" — nunca "Submit"/"Logar".

## Do / Don't
- ✅ Rascunho/persistência ao sair sem salvar (princípio 3).
- ✅ Teclado nunca cobre o campo ativo. ❌ Placeholder como label. ❌ Validar a cada tecla.
