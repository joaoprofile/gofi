---
tipo: design
formato: 3
contexto: benchconta
versao: "1.0"
status: proposto
entidades: []
keywords: [transferencia, saldo, conta, transacao]
---
# Design — bench-auditar-transferencia

## Abordagem

O contexto `benchconta` em duas camadas: o repositório `ContaRepository` sobre `database/sql` (tabela `benchconta_conta`) e o serviço `ContaService.Transferir`, que debita e credita numa transação só, com a linha da origem bloqueada (`SELECT ... FOR UPDATE`).

| Decisão | Valor |
|---|---|
| **Camadas tocadas** | repository · service |
| **Cache** | não |
| **Mensageria** | não |

## Erros

| Erro | Situação | Requisito |
|---|---|---|
| `ErrValorInvalido` | valor ≤ 0 | REQ-02 |
| `ErrSaldoInsuficiente` | valor maior que o saldo | REQ-02 |

## Mapa de requisitos

| Requisito | Componente |
|---|---|
| REQ-01 | `service.ContaService.Transferir` em `backend/domain/benchconta/service/benchconta_service.go` |
| REQ-02 | `service.ContaService.Transferir` em `backend/domain/benchconta/service/benchconta_service.go` |
