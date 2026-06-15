# Naming Conventions — Go

## Arquivos
- `snake_case.go` (ex: `user_repository.go`, `query_dto.go`)
- Test files: `{nome}_test.go` (mesmo nome do arquivo testado + `_test`)

## Pacotes
- `lowercase`, sem underscore (ex: `model`, `service`, `repository`)
- O nome do pacote deve refletir o que ele exporta, não o que ele faz

## Tipos e Interfaces
- `PascalCase` (ex: `UserService`, `OrderRepository`)
- Interface não recebe sufixo `Interface` ou prefixo `I` — o nome **é** a interface

## Variáveis e funções
- `camelCase` (ex: `userID`, `findByEmail`)
- Receivers: 1-2 letras minúsculas, consistentes no tipo (ex: `s *userService`, `r *userRepository`)

## Constantes de erro
- `ErrContextAction` (ex: `ErrPersonCreate`, `ErrOrderNotFound`)
- Variáveis de erro no padrão `ErrXxxXxx` em `service/errors.go`, registradas com `errs.Register*`

## SQL e domínio
- **Tabela SQL:** singular, snake_case, em inglês (ex: `user`, `order`, `invoice_item`) — **nunca plural**
- **Colunas SQL:** snake_case, em inglês (ex: `tenant_id`, `created_at`, `unit_price`)
- **Campos de entidade Go:** PascalCase, em inglês (ex: `TenantID`, `CreatedAt`, `UnitPrice`)
- **Tags `db`:** snake_case, igual à coluna (ex: `db:"tenant_id"`)
- **Tags `json`:** camelCase para o cliente (ex: `json:"tenantId"`)

## Códigos de moeda e país no retorno da API
- **Moeda** no objeto de resposta da API é **sempre** `json:"currencyCode"` —
  nunca `currency`, `currencyId`, `coin` ou `moeda`. Carrega o código ISO 4217
  (ex: `"USD"`, `"BRL"`, `"MXN"`).
- **País** no objeto de resposta da API é **sempre** `json:"countryCode"` —
  nunca `country`, `countryId` ou `pais`. Carrega o código ISO 3166-1 alpha-2
  (ex: `"US"`, `"BR"`, `"MX"`).
- Vale para **todo** DTO de resposta que exponha código de moeda ou país; o
  sufixo `Code` deixa explícito ao cliente que o valor é um código padronizado,
  não um nome/label (a tradução para nome legível é responsabilidade do
  frontend, como nos enums).

## Endpoints
- Em inglês, snake_case ou plural simples no path (ex: `/users`, `/orders/{id}/items`)
- DTOs em inglês: `CreateUserRequest`, `UpdateOrderRequest`

## Imports
- Sempre agrupar em 3 blocos: stdlib → gofi → externos → internos do projeto
- Alias **somente** quando há colisão de nomes entre pacotes
