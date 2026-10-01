# Regras de qualidade da doc gerada

## Regras de qualidade

- **Manual, não tratado.** Listas numeradas, tabelas, mocks colados. **Zero**
  prosa explicativa, **zero** "este endpoint foi desenhado para…". Se uma
  seção tem mais de 2 parágrafos de texto corrido, ela está errada —
  converta em lista ou tabela.
- **Toda chamada documentada tem mock colável.** Request real (URL, método,
  headers obrigatórios, body com valores plausíveis) + response real. Mocks
  parciais (só body, sem headers; ou só "exemplo de resposta sem campos")
  não passam.
- **Filtro dinâmico merece a §4 dedicada** (layout em `template-frontend.md`) quando o endpoint usa filtro
  dinâmico do SDK. Obrigatório: passo-a-passo de como montar, shape do
  request, tabela de operadores por tipo de campo, tabela de campos
  permitidos (do `AllowedFields`), **mínimo 5 mocks prontos** cobrindo:
  filtro vazio, single, multi (`IN`), composto com `AND`, `OR`, `BETWEEN`,
  `IS_NULL`. Cada `field` listado em `AllowedFields` aparece na tabela §4.4
  com seu `FilterType` e seus operadores válidos.
- **Exemplos JSON obrigatórios** — nunca `"campo": "<valor>"`. Usar UUIDs
  v7 plausíveis, timestamps ISO-8601, inteiros na faixa válida.
- **Tipos TS refletem o JSON exatamente**: ponteiro no modelo → `T | null`,
  tipo de data → `string` (ISO-8601), inteiros → `number`, decimais/float →
  `number` (alertar se precisão importa).
- **Tabela de erros lista TODOS os erros do `errors.go`** (service e
  application) que este handler pode disparar — não só os comuns. Em
  Template B vira matriz de teste.
- **"Armadilhas conhecidas" documenta comportamento do service**, não
  apenas o contrato. Regras que modificam estado de forma inesperada
  (reset silencioso de campo, preservação de valor antigo, default
  condicional) entram aqui.
- **Autenticação é específica:** liste exatamente quais claims o backend
  consome — frontend não deve adivinhar. Os nomes vêm do middleware real.
- **Fluxo de tela mostra ordem das chamadas** + qual resposta reutilizar
  (evitar re-fetch após 200 de mutação).
- **Nunca documentar campos que não existem no código.** DTO interno ≠
  response.
- **Paginação — sempre ler o tipo real do SDK.** Confirme os nomes dos
  campos do envelope no tipo de paginação em `.gofi/gofi-sdk-{lang}/` (e/ou
  na knowledge). Não assuma nomes — eles podem mudar.
- **Campos comentados (comentário no código, `--` no SQL) não existem na
  resposta.**
- **Datas: documentar o parser exato.** Se o handler usa um parser estrito
  (ex.: RFC3339 sem milissegundos), documentar em validação + Armadilhas +
  snippet de conversão `d.toISOString().replace(/\.\d{3}Z$/, 'Z')`. Confirme
  o parser real antes de afirmar.
- **Para QA (Template B): cada caso de erro tem `Setup` reproduzível**
  (estado do banco a montar) e **`Pós-condição verificável`** (query SQL
  ou GET subsequente que confirma o efeito).
- **Smoke test do Template B é colável** — sequência pronta para
  Postman/Bruno/curl, sem placeholders sem valor sugerido.

