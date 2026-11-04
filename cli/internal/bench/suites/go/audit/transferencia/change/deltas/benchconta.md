---
tipo: delta
formato: 3
contexto: benchconta
versao: "1.0"
status: proposto
keywords: [transferencia, saldo, conta, transacao]
---
# Delta — benchconta — bench-auditar-transferencia

## Propósito

A loja move crédito entre contas de clientes e não pode perder nem criar dinheiro no caminho.

## ADICIONADO

### Requisito: REQ-01 — Transferência atômica
O sistema DEVE transferir um valor em centavos (inteiro) de uma conta para outra de forma atômica: ou as duas contas mudam, ou nenhuma; duas transferências ao mesmo tempo da mesma conta nunca a deixam com saldo negativo.

#### Cenário: REQ-01.1 — Transferência simples
- DADO a conta A com 1000 centavos e a conta B com 0
- QUANDO A transfere 300 para B
- ENTÃO A fica com 700 e B com 300

#### Cenário: REQ-01.2 — Falha no meio
- DADO a conta A com 1000 centavos e uma falha ao creditar B
- QUANDO A transfere 300 para B
- ENTÃO A continua com 1000 e B com 0

### Requisito: REQ-02 — Transferência recusada
O sistema NÃO DEVE transferir um valor menor ou igual a zero (erro de valor inválido) nem um valor maior que o saldo da origem (erro de saldo insuficiente).

#### Cenário: REQ-02.1 — Valor zero
- DADO a conta A com 1000 centavos
- QUANDO A transfere 0 para B
- ENTÃO recebe recusa por valor inválido

#### Cenário: REQ-02.2 — Saldo insuficiente
- DADO a conta A com 100 centavos
- QUANDO A transfere 300 para B
- ENTÃO recebe recusa por saldo insuficiente
