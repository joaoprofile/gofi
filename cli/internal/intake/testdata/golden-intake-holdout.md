# Golden de controle — entrada determinística

Casos nunca usados para ajustar as regras: medem se o intake generaliza além
do golden principal. Mesmo projeto de teste.

| # | Pedido | Contexto | Plano | Pergunta |
|---|---|---|---|---|
| H01 | quero um PRD para devolução de pedidos | novo | gofi-pd | sim |
| H02 | mude a regra de preço mínimo do pricing | pricing | gofi-spec → gofi-eng → gofi-qa | não |
| H03 | tem um bug quando o pedido é faturado duas vezes | order | gofi-eng → gofi-qa | não |
| H04 | gere a documentação do catálogo | catalog | gofi-doc | não |
| H05 | em que pé está o pricing? | pricing | — | não |
| H06 | valide a implementação do cancelamento de pedidos | order | gofi-qa | não |
| H07 | crie a especificação de notificações | novo | gofi-pd → gofi-spec | não |
| H08 | adicione um endpoint de exportação no catálogo | catalog | gofi-spec → gofi-eng → gofi-qa | não |
