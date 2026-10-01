# Engenharia reversa — spec a partir do código

Ativa quando o usuário pede explicitamente. **Nunca altere o código** —
você apenas lê e produz/atualiza a spec.

## Modo 1 — Atualizar spec existente pelo código
1. Ler arquivos indicados (model, repository, service, handler)
2. Comparar com a spec — listar divergências
3. Atualizar **somente** as seções afetadas (§0.1, §4, §8, §10, Histórico)
4. Bump de versão e entrada no Histórico

## Modo 2 — Criar spec do zero pelo código
1. **Levantar o pacote pelo grafo antes de abrir arquivo** — `gofi show
   <pacote> -n 40` lista os símbolos que ele contém e quem depende dele;
   `gofi show <Símbolo>` dá assinatura, doc e vizinhança de cada um.
   Isso diz **quais** arquivos importam
2. Ler os arquivos do pacote que o grafo apontou — aqui a leitura é integral,
   porque tag de struct, status code e texto de erro só o arquivo tem
3. Derivar manifesto, entidade, DTOs, contratos, endpoints, erros, regras de negócio
4. Gerar spec completa seguindo o template
5. Não invente nada que não esteja no código

## Modo 3 — Mapa de contextos de um repositório adotado

Ativa depois de um `gofi init` sobre uma base de código que já existia: há
código, mas nenhum `specs/`, nenhum `memory/contexts/` e nenhuma diretiva
`//gofi:context` — o grafo enxerga as chamadas e não sabe a que contexto cada
símbolo pertence. Este modo produz **o mapa**; quem escreve a diretiva no código
é o `/gofi-eng` (nunca esta skill).

1. **Levante os candidatos pelo grafo, não varrendo a árvore.**
   `gofi_graph_index.json` → o `gofi_graph_report.md` **de cada escopo** (o
   índice diz a pasta: backend em `.`, cada superfície em `{nome}/`; o escopo
   `sdk` fica de fora — é dependência, não contexto). As **comunidades** do
   relatório são o candidato natural a contexto (é um agrupamento por
   acoplamento real, não por pasta); os **pontos centrais** apontam o que é
   infraestrutura compartilhada e por isso **não** é contexto; as **conexões
   inesperadas** e os **ciclos** marcam as fronteiras que hoje vazam.
2. **Cruze com a estrutura de pastas.** Quando comunidade e pasta concordam, o
   contexto é óbvio. Quando divergem, a divergência é o achado — reporte-a, não
   a resolva sozinho.
3. **Proponha a tabela e pare.** Uma linha por contexto candidato:

   | Contexto proposto | Pacotes/pastas | Evidência | Dúvida |
   |---|---|---|---|
   | `billing` | `internal/invoice`, `internal/payment` | comunidade 2; `Invoice` é ponto central | `payment` é contexto próprio? |

   **A fronteira de contexto é decisão do dev, não sua.** Você traz a evidência
   e a leitura; o nome e o recorte final são confirmados por quem conhece o
   negócio. Só siga depois do "ok".
4. **Emita, por contexto confirmado:** `memory/contexts/{contexto}.md` (com o
   `## Estado atual` declarando **quais pacotes pertencem ao contexto** — é essa
   lista que o `/gofi-eng` aplica) e, quando o dev pedir a spec daquele
   contexto, `specs/{contexto}/sdd-{contexto}.md` pelo **Modo 2**. Adotar um
   repo não exige spec de tudo de uma vez: o mapa vale por si, e as specs vêm
   por contexto, sob demanda.
5. **Frontmatter do contexto adotado:** `status: implementado`,
   `versao_spec: n/a` enquanto não houver spec — o código existe e é essa a
   verdade que o índice deve mostrar.
6. **Regenerar os índices** (`gofi index docs`) e
   devolver, nos "Próximos passos", que falta o `/gofi-eng` gravar as
   diretivas.

