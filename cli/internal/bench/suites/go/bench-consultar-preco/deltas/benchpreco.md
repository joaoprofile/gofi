---
tipo: delta
formato: 3
contexto: benchpreco
versao: "1.0"
status: proposto
keywords: [preço, consulta, http, rota]
---
# Delta — benchpreco — bench-consultar-preco

## Propósito

O aplicativo precisa consultar por HTTP o preço final de um produto com o desconto da sua categoria.

## ADICIONADO

### Requisito: REQ-01 — Preço com desconto da categoria
O sistema DEVE receber o preço em centavos (inteiro) e a categoria (comum, promocional) e devolver o preço final: igual ao preço na comum, 20% a menos na promocional, arredondando o desconto para baixo ao centavo; recusa categoria desconhecida ou preço negativo com um erro de entrada inválida.

#### Cenário: REQ-01.1 — Categoria comum
- DADO um preço de 1000 centavos na categoria comum
- QUANDO o cálculo é feito
- ENTÃO o preço final é 1000 centavos

#### Cenário: REQ-01.2 — Categoria promocional
- DADO um preço de 999 centavos na categoria promocional
- QUANDO o cálculo é feito
- ENTÃO o preço final é 800 centavos

#### Cenário: REQ-01.3 — Categoria desconhecida
- DADO um preço de 1000 centavos na categoria vip
- QUANDO o cálculo é feito
- ENTÃO recebe recusa por entrada inválida

#### Cenário: REQ-01.4 — Preço negativo
- DADO um preço de -1 centavo na categoria comum
- QUANDO o cálculo é feito
- ENTÃO recebe recusa por entrada inválida

### Requisito: REQ-02 — Consulta HTTP do preço
O sistema DEVE responder `GET /precos?valor=<centavos>&categoria=<categoria>` com 200 e o preço final em JSON `{"preco": <centavos>}`, e com 400 quando a consulta for recusada pelo REQ-01 ou o valor não for um inteiro.

#### Cenário: REQ-02.1 — Consulta válida
- DADO a consulta `valor=999&categoria=promocional`
- QUANDO a rota é chamada
- ENTÃO responde 200 com `{"preco": 800}`

#### Cenário: REQ-02.2 — Valor que não é número
- DADO a consulta `valor=abc&categoria=comum`
- QUANDO a rota é chamada
- ENTÃO responde 400
