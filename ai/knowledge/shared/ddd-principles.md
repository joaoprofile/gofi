# Princípios de DDD — universais

Cross-agent, cross-language. Princípios que `gofi-spec` e `gofi-eng`
respeitam ao modelar e implementar.

## Agregado e raiz

- Cada contexto tem **uma entidade raiz** que é a única porta de entrada para
  alterações no agregado.
- Entidades filhas (ex: `OrderItem` dentro de `Order`) são modificadas pela
  raiz; nunca expostas diretamente em endpoints.
- Operações que cruzam agregados acontecem por **eventos de domínio**, não
  por chamadas síncronas em cascata.

## Value Objects

- Quando dois ou mais campos têm sentido **somente juntos** (ex: `Money{Amount,
  Currency}`, `Address{Street, City, ZipCode}`, `Pricing{Price, Discount}`)
  modele como Value Object.
- Identidade do VO é o **valor** — dois VOs com mesmos campos são iguais.
- VOs são **imutáveis**: para "alterar", crie um novo.

Mapping para o banco:
- **Múltiplas colunas** (uma por sub-campo) — útil quando o VO é consultado/
  filtrado por campos individuais. O ORM do gofi expande recursivamente
  structs aninhadas com tag `db`.
- **Coluna única** (JSON/texto) — útil quando o VO é tratado como blob e raramente
  consultado por campo. Implementa `Scanner`/`Valuer` (Go) ou equivalente.

## Linguagem ubíqua

- Termos do domínio devem ser **consistentes** em todos os artefatos: PRD,
  spec, código, banco, API.
- Glossário do projeto vive em `knowledge/shared/glossario.md` (`gofi train --shared`).
- Em discussão com o usuário, use os mesmos termos do glossário; quando houver
  ambiguidade, peça definição.

## Invariantes

Documentar **o que nunca pode acontecer** no domínio:
- "Email duplicado por tenant"
- "Saldo negativo"
- "Status retroativo (de active para draft)"

Cada invariante vira pelo menos uma regra de negócio (RN-XX) na spec e uma
verificação no service.

## Ciclo de vida (state machine)

Quando a entidade tem estado, mapeie a transição completa:

```
draft → active → inactive
       ↓
   cancelled (terminal)
```

A spec documenta as transições válidas; o service rejeita transições inválidas
com erro de validação.
