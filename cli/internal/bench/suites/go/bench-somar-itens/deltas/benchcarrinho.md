---
tipo: delta
formato: 3
contexto: benchcarrinho
versao: "1.0"
status: proposto
keywords: [carrinho, soma, itens, centavos]
---
# Delta — benchcarrinho — bench-somar-itens

## Propósito

A loja precisa do subtotal do carrinho a partir do preço e da quantidade de cada item.

## ADICIONADO

### Requisito: REQ-01 — Subtotal do carrinho
O sistema DEVE receber os itens do carrinho, cada um com preço unitário em centavos (inteiro) e quantidade (inteiro), e devolver a soma de preço × quantidade de todos, em centavos; um carrinho sem itens tem subtotal 0.

#### Cenário: REQ-01.1 — Dois itens
- DADO um item de 1500 centavos com quantidade 2 e um de 990 centavos com quantidade 1
- QUANDO o cálculo é feito
- ENTÃO o subtotal é 3990 centavos

#### Cenário: REQ-01.2 — Carrinho vazio
- DADO um carrinho sem itens
- QUANDO o cálculo é feito
- ENTÃO o subtotal é 0 centavos

### Requisito: REQ-02 — Item inválido
O sistema NÃO DEVE calcular quando algum item tiver preço negativo ou quantidade menor que 1: recusa com um erro de item inválido.

#### Cenário: REQ-02.1 — Preço negativo
- DADO um item de -1 centavo com quantidade 1
- QUANDO o cálculo é feito
- ENTÃO recebe recusa por item inválido

#### Cenário: REQ-02.2 — Quantidade zero
- DADO um item de 1000 centavos com quantidade 0
- QUANDO o cálculo é feito
- ENTÃO recebe recusa por item inválido
