# Pattern — Feedback mobile

Espelha o [web](../../../web/gofi-ui/patterns/feedback.md) com canais mobile.

## Canal por situação
| Situação | Canal |
|----------|-------|
| Confirmação leve ("Salvo") | [Toast](../components/toast.md) (com Desfazer se reversível) |
| Aviso/erro de seção | banner inline na tela |
| Erro de campo | inline no [Field](../components/field.md) |
| Erro de carregamento | banner + retry (e pull-to-refresh) |
| Decisão destrutiva | [Modal/Confirm](../components/modal.md) ou action sheet |
| Em progresso | [Skeleton](../components/skeleton.md)/spinner, botão `loading` |

## Destrutivo: confirmação **ou** undo (nunca ambos ausentes)
- Reversível → executa + **Desfazer** no toast (princípio 10).
- Irreversível → confirm com impacto explícito + botão `danger` (verbo específico).
- iOS: `ActionSheetIOS`/Alert nativo aceitável para destrutivo simples.

## Toque e tempo
- Feedback de toque < 100ms (`Pressable` pressed). Otimista quando seguro, com rollback.
- Háptica leve em confirmações/erros (opcional, respeita preferências do sistema).

## Acessibilidade
- Toast → `announceForAccessibility`. Erro de submit leva foco ao 1º inválido.
- Não comunicar só por cor; ícone + texto sempre.

## Do / Don't
- ✅ "Desfazer" > "tem certeza?" para reversível. ✅ Erro diz o quê + o que fazer.
- ❌ Toast atrás do tab bar/notch. ❌ Sucesso sem confirmação visível/audível.
