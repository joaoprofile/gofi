---
tipo: delta
formato: 3
contexto: benchreembolso
versao: "1.0"
status: proposto
keywords: [reembolso, taxa, prazo, centavos]
---
# Delta — benchreembolso — bench-auditar-reembolso

## Propósito

Calcula o reembolso de um pedido cancelado, em centavos.

## ADICIONADO

### Requisito: REQ-01 — Reembolso menos a taxa
O sistema DEVE devolver o valor pago, em centavos (inteiro), menos uma taxa de 2% dele, arredondada para baixo ao centavo.

#### Cenário: REQ-01.1 — Taxa de 2%
- DADO um pagamento de 10000 centavos, pedido há 5 dias
- QUANDO o reembolso é calculado
- ENTÃO o reembolso é 9800 centavos

### Requisito: REQ-02 — Valor negativo
O sistema NÃO DEVE calcular o reembolso de um valor pago negativo: recusa com um erro de valor inválido.

#### Cenário: REQ-02.1 — Valor negativo
- DADO um pagamento de -1 centavo, pedido há 5 dias
- QUANDO o reembolso é calculado
- ENTÃO recebe recusa por valor inválido

### Requisito: REQ-03 — Prazo do reembolso
O sistema DEVE recusar o reembolso de um pedido de mais de 30 dias com um erro de prazo expirado; até 30 dias, inclusive, reembolsa.

#### Cenário: REQ-03.1 — Fora do prazo
- DADO um pagamento de 10000 centavos, pedido há 31 dias
- QUANDO o reembolso é calculado
- ENTÃO recebe recusa por prazo expirado
