# Alteração em contexto já implementado — análise de impacto obrigatória

Quando a tarefa **edita um contexto que já está implementado** (em vez de criar
um do zero), o risco dominante não é o código novo — é o que você **quebra sem
ver** em consumidores existentes. Antes de fechar a edição, faça uma análise de
impacto detalhada e cubra os **relacionados**, não só o arquivo aberto.

## Gatilho

Qualquer mudança em um **artefato compartilhado** entre ≥2 contextos/camadas:

- struct de `model/` (entity, DTO, VO) reusada por outro contexto
- enum/constante de `services/common/enums/` ou `kafka.Type*`
- assinatura de interface (service, repository, bridge) consumida em mais de um lugar
- coluna/tabela tocada por migration (outros repos podem ler/escrever nela)
- helper compartilhado (`services/common/helpers/`, helper model-bound)

## Procedimento

1. **Mapeie os consumidores antes de editar.** Primeiro `gofi show <símbolo>`
   (quem chama, tipos que toca); depois a busca literal em todo o módulo, não
   só no pacote atual, para os usos que não são chamada — cada linha já vem sob
   o símbolo que a contém:
   - tipo: `gofi show {Type}` + `gofi find --regex "model\.{Type}\b" --in code`
     + `gofi find --regex "FindFromCriteria\[.*{Type}\]" --in code` (instanciação genérica)
   - enum/const: `gofi find --text "{Const}"` (achar todo `switch`/`map` que precisa do valor novo)
   - interface: `gofi show {Interface}` sobre grafo `--deep` (todo struct que a
     implementa) + `gofi find --text "{Interface}" --in code` (mocks, asserções) —
     texto não resolve implementação
   - coluna: `gofi find --text "{column}" --in code` nos `*SelectFields` e SQL de todos os repos
2. **Classifique cada consumidor**: continua válido / precisa ajuste / quebra.
3. **Ajuste todos na mesma entrega.** Mock de repo que ganhou método, `switch`
   que ganhou caso, `SELECT` que precisa da coluna nova — tudo junto.
4. **Build + test dos pacotes consumidores**, não só do editado.
   `go -C {path} build ./...` e `go -C {path} test ./{consumidores}/...`.
5. Se a mudança for grande/incerta, vale rodar um fan-out (subagentes) para
   varrer consumidores em paralelo antes de tocar o código.

## Casos clássicos de quebra silenciosa (sem erro de compilação)

- **Scan posicional do `sqln`** — adicionar/reordenar campo `db` numa struct
  reusada por N repos quebra **cada** `SELECT` que escaneia ela, por aridade ou
  desalinhamento. Só estoura em runtime. Detalhe e regras em
  `.claude/sdk/go/knowledge/value-objects.md` §"O contrato posicional é do TIPO,
  não do repo".
- **Coluna que veio de `JOIN`** — o campo novo na struct exige o `.Join(...)`
  em **todas** as queries que materializam o tipo; faltar o join = coluna
  ausente = aridade quebrada.
- **Enum novo sem destino** — valor adicionado num enum compartilhado sem
  atualizar os `switch`/`map` consumidores → cai no `default` silencioso.
- **Migration que altera tabela de outro dono** — outro contexto que lê/escreve
  a mesma tabela pode depender da forma antiga.

## Princípio

O símbolo compartilhado tem um **contrato implícito com todos os seus
consumidores**. Editar o dono sem varrer os consumidores transfere o custo para
o runtime de produção. A varredura é barata; o incidente não.
