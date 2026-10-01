# Regras universais — núcleo (contexto, testes, spec, clean code)

## Regras universais (cross-language) — núcleo

Aplicam-se em todo contexto, em qualquer linguagem suportada:

- **Todo pacote de um contexto nasce com `//gofi:context {contexto}`.** Os
  pacotes são por camada (`model`, `service`, `repository`, `handler`), então é
  a diretiva — não o nome do pacote — que diz a **qual contexto** o símbolo
  pertence. É o único elo entre o grafo e `specs/{contexto}/` +
  `.claude/memory/contexts/{contexto}.md`; sem ela o agent seguinte volta a
  adivinhar pelo nome da pasta.

  ```go
  //gofi:context billing
  package model
  ```

  Na cláusula `package` basta em **um** arquivo do pacote — vale para todos os
  símbolos dele. O nome é **o mesmo** de `specs/{contexto}/`, kebab-case, sem
  variação. Um símbolo que serve outro contexto leva a diretiva na própria
  declaração, sobrescrevendo a do pacote. Hoje só o extractor Go lê a diretiva;
  nas demais linguagens ela é documentação até o extractor passar a lê-la.
- **Editar contexto implementado → análise de impacto nos consumidores antes de fechar.**
  Mudança em artefato compartilhado (struct de `model/` reusada por outro
  contexto, enum/`kafka.Type*`, interface, coluna de migration, helper comum)
  carrega contrato implícito com **todos** os consumidores. Levante os
  consumidores pelo grafo — `gofi show <símbolo>` responde "quem chama"
  sem varrer a árvore, e a conclusão só é confiável com o escopo em modo `deep`
  (confira o `mode` no índice; `gofi index code` sem `--deep` só é `deep` se o `.gofi.yaml` declarar `graph.deep: true`). `gofi find --text`/`--regex`
  é o fallback declarado: linguagem sem extractor, ou alvo que não é símbolo.
  Classifique cada consumidor (válido/ajuste/quebra), ajuste todos na mesma
  entrega e rode `build`+`test` dos pacotes **consumidores**.
  Caso recorrente e mais perigoso: o **scan posicional do `sqln`** —
  `FindFromCriteria[T]` escaneia por `GetMappedCols(&T)` (folhas `db` na ordem
  de declaração), independente da string do `Select(...)`; logo um `model.{Type}`
  materializado por ≥2 repos exige atualizar **cada** `SELECT` em contagem E
  ordem ao mudar a struct — um deles ficar para trás quebra só naquele caminho,
  em runtime (`expected N destination arguments in Scan, not M` ou
  desalinhamento silencioso). Sub-caso campeão: **campo struct para coluna JSONB
  sem `sql.Scanner`** — o mapper expande a struct e, se os sub-campos não têm tag
  `db`, ela contribui **0 folhas** → aridade curta em toda leitura do tipo (o
  `INSERT` engana porque passa `[]byte` explícito). Coluna JSONB = tipo com
  `Scan`/`Value`, nunca struct recursiva. `build`/`test` **sem DB não pega** —
  validar `len(mapping.GetMappedCols(&T{}))` == colunas do `SELECT` ou exercitar
  a query. Procedimento completo em
  `.claude/expertise/ddd-architecture/impact-analysis-on-change.md`; mecânica do scan em
  `.claude/sdk/go/knowledge/value-objects.md` §"O contrato posicional é do TIPO,
  não do repo".
- **Mocks são handcraft.** Mock de repository com fn fields no service test;
  stub de service com campos de retorno no handler test. Sem frameworks de
  mock externos. Mock implementa todos os métodos da interface, incluindo
  os de cleanup.
- **Contratos atualizados → testes atualizados** na mesma entrega:
  - Novo método no service → cobrir em `service_test.go`
  - Novo método no repository → adicionar ao mock no `service_test.go`
  - Nova rota no handler → cobrir em `handler_test.go`
- **Bug fix / melhoria → teste de regressão na mesma entrega (LEI).**
  Toda correção de bug e toda melhoria de comportamento vem acompanhada de um
  teste que **falha sem o fix e passa com ele**, ancorado no cenário exato que
  estava quebrado (input que reproduzia o bug → assert do comportamento
  correto). Preferir escrever o teste **antes** do fix (red→green). O teste
  vai na camada onde o defeito mora (service/handler/repository/adapter) e roda
  verde no `test` do pacote. Fix sem teste de regressão é entrega incompleta —
  nunca empurrar pro QA nem deixar "pra depois".
- **Adapters de SDK externo** vão em `adapter/`, nunca dentro de `repository/`.
- **Spec é fonte da verdade**: o que não está na spec não é implementado;
  divergências durante a implementação são registradas no Histórico.
- **Clean code — código fala por si; comentários só quando WHY for não-óbvio.**
  Default: **zero comentários**. Identificadores bem escolhidos + funções
  pequenas + early returns substituem narração. Antes de escrever um
  comentário: renomeie a variável, extraia uma função, mova a lógica.
  Quando comentar for inevitável: **uma linha**, lidera com WHY (constraint
  não-óbvio, workaround documentado, decisão de negócio que o código não
  carrega, invariante de segurança) — **sucinto, uma linha; duas é o teto**.
  Comentário que vira parágrafo pertence à spec ou à memória, não ao código.
  **Nunca** comente campo de struct (anotar atributo por atributo polui o tipo
  e some com a forma dele — se um campo esconde constraint real, uma linha
  acima do tipo nomeando esse campo). **Nunca** narre o WHAT (`// loads
  user`), nunca documente o óbvio (`// Pool struct represents a pool`),
  nunca deixe TODO sem ação concreta + condição clara
  (`// TODO(rbac-fino): trocar quando user roles forem fine-grained`),
  nunca marque seções com banners (`// --- helpers ---`). Princípios
  completos e exemplos em `.claude/expertise/ddd-architecture/clean-code.md`.

Para regras language-specific (ex: `nunca fmt.Println`, `nunca *sql.DB fora
do sqln`, etc.), consulte
`.claude/sdk/<lang>/knowledge/absolute-rules.md`.
