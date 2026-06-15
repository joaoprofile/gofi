# Pattern — Feedback e confirmação

Como o sistema responde às ações e protege o usuário de lapsos (princípios 3
e 10).

## Canal por situação
| Situação | Canal |
|-----------|---------|
| Confirmação leve de ação ("Salvo") | [Toast](../components/toast-banner.md) (com Desfazer se reversível) |
| Aviso/erro persistente de seção | [Banner](../components/toast-banner.md) |
| Erro de campo | inline no [Field](../components/field.md) |
| Erro técnico de carregamento | Banner + tentar de novo no lugar do conteúdo |
| Decisão destrutiva | [Confirm Dialog](../components/modal-drawer.md) |
| Operação em andamento | [Progress](../components/progress.md)/[Skeleton](../components/skeleton-spinner.md), botão `loading` |

## Destrutivo: confirmação **ou** desfazer (nunca os dois ausentes)
- **Reversível** → executar agora + **Desfazer** no toast (o melhor fluxo; princípio 10).
- **Irreversível** → um Confirm Dialog com o impacto explícito + um botão `danger`
  com um verbo específico ("Excluir"), não "OK".

## Timing e atenção
- Resposta visual < 100ms ao toque; otimista quando for seguro (atualizar a UI e
  reconciliar, com rollback em caso de erro).
- Um toast dura o tempo suficiente para ler, pausa no foco; não é o **único** canal
  para informação crítica.

## Acessibilidade
- `role="status"`/`alert` conforme a urgência; um erro de submit move o foco para o primeiro inválido.
- Não transmitir só pela cor (ícone + texto sempre).

## Do / Don't
- ✅ "Desfazer" > "tem certeza?" para ações reversíveis.
- ✅ A mensagem de erro diz **o que** aconteceu e **o que fazer**.
- ❌ O alert nativo do navegador para um fluxo de produto. ❌ Sucesso sem confirmação visível.
