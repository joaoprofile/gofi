---
tipo: delta
formato: 3
contexto: benchestoque
versao: "1.0"
status: proposto
keywords: [estoque, reserva, quantidade]
---
# Delta — benchestoque — bench-lacuna-contrato

## Propósito

A loja reserva unidades do estoque ao montar um pedido e precisa saber o saldo depois da reserva.

## ADICIONADO

### Requisito: REQ-01 — Saldo após a reserva
O sistema DEVE receber o estoque disponível (inteiro) e a quantidade reservada (inteiro) e devolver o saldo, estoque menos reserva, recusando a reserva maior que o estoque com um erro de estoque insuficiente.

#### Cenário: REQ-01.1 — Reserva parcial
- DADO um estoque de 10 e uma reserva de 3
- QUANDO o cálculo é feito
- ENTÃO o saldo é 7

#### Cenário: REQ-01.2 — Reserva maior que o estoque
- DADO um estoque de 2 e uma reserva de 5
- QUANDO o cálculo é feito
- ENTÃO recebe recusa por estoque insuficiente

### Requisito: REQ-02 — Entrada inválida
O sistema NÃO DEVE calcular quando o estoque ou a reserva forem negativos: recusa com um erro de entrada inválida.

#### Cenário: REQ-02.1 — Reserva negativa
- DADO um estoque de 10 e uma reserva de -1
- QUANDO o cálculo é feito
- ENTÃO recebe recusa por entrada inválida
