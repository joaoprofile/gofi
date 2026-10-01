# Golden — entrada determinística (`gofi intake`, só regras)

Casos de regressão do intake, medidos por `golden_test.go` contra o projeto de
teste de `intake_test.go`: `order` implementado (spec e código), `pricing` com
spec e código, `integration`, `invoice`, `shipping` com uma spec cada, `catalog`
só com PRD. O léxico do projeto diz pedido = order, catálogo = catalog,
preço = pricing.

| # | Pedido | Contexto | Plano | Pergunta |
|---|---|---|---|---|
| I01 | crie um PRD de sincronização de pedidos | novo | gofi-pd | sim |
| I02 | altere o fluxo de pricing, adicionando um rate limit no intervalo dos envios de preços | pricing | gofi-spec → gofi-eng → gofi-qa | não |
| I03 | corrija o bug no cancelamento de pedidos | order | gofi-eng → gofi-qa | não |
| I04 | documente a API de pedidos | order | gofi-doc | não |
| I05 | audite o contexto de pricing | pricing | gofi-qa | não |
| I06 | como está o contexto de pedidos? | order | — | não |
| I07 | /gofi-eng implemente o cancelamento | — | direto | não |
| I08 | especifique o catálogo | catalog | gofi-spec | não |
| I09 | implemente o catálogo | catalog | gofi-spec → gofi-eng → gofi-qa | não |
| I10 | crie o módulo de programa de fidelidade | novo | gofi-pd → gofi-spec → gofi-eng → gofi-qa | não |
| I11 | altere as regras | — | gofi-spec → gofi-eng → gofi-qa | sim |
| I12 | revise o código de pricing | pricing | gofi-qa | não |
| I13 | crie a tela de cancelamento de pedidos | order | gofi-spec → gofi-ui → gofi-qa | não |
| I14 | panorama dos contextos | — | gofi-status | não |
| I15 | provisione o ambiente de pricing | pricing | gofi-ops | não |
| I16 | corrija o erro no envio de preços | pricing | gofi-eng → gofi-qa | não |
