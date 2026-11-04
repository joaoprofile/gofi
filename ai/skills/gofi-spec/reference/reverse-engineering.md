# Engenharia reversa — baseline da spec viva a partir do código

Ativa quando o usuário pede, ou quando um contexto roda sem spec viva. **Nunca
altere o código** — você só lê e escreve a spec.

## Modo 1 — Código e spec viva divergem

O código faz algo que a spec viva não diz (ou o contrário). A spec viva **não
é editada**: a divergência vira uma **mudança**.

1. Leve a divergência pelo grafo e pelos testes: `gofi show <símbolo>`, os
   testes que o exercitam.
2. Decida com a pessoa qual lado está certo:
   - **o código está certo** → mudança com delta MODIFICADO/ADICIONADO que
     descreve o comportamento real, `design: n/a` se nenhum código muda;
   - **a spec está certa** → é um defeito: mudança sem delta de requisito não
     existe — registre o achado para o `gofi-eng` corrigir contra o requisito
     que já está lá.
3. `gofi spec check <id>` e entregue para aprovação.

## Modo 2 — Baseline: spec viva de um contexto que roda sem spec

1. **Levante o contexto pelo grafo antes de abrir arquivo** — `gofi show
   ctx:{contexto}`, `gofi show <pacote> -n 40` (símbolos e dependentes),
   `gofi show <Símbolo>` (assinatura, doc, vizinhança). Isso diz **quais**
   arquivos importam.
2. Leia os arquivos que o grafo apontou e **os testes**: o teste é a melhor
   fonte de cenário — cada caso de teste é um DADO/QUANDO/ENTÃO candidato.
3. Escreva `specs/{contexto}/spec.md` pelo `.claude/templates/spec-template.md`:
   **só o quê** — requisitos `REQ-NN` (capacidades e invariantes observáveis) e
   cenários. Nada de tabela, rota, tipo ou arquivo: o como já está no código.
4. **Não invente nada que não esteja no código.** Comportamento que o código
   tem e nenhum teste cobre entra como requisito com cenário e a nota
   `[PRECISA ESCLARECER: confirmar que este é o comportamento desejado]` — a
   pessoa decide antes de aprovar.
5. `gofi spec check {contexto}` em zero erros; a pessoa aprova com
   `gofi spec approve {contexto}`. A partir daí, o contexto muda só por mudanças.

## Modo 3 — Mapa de contextos de um repositório adotado

Ativa depois de um `gofi init` sobre uma base de código que já existia: há
código, mas nenhum `specs/`, nenhum `memory/contexts/` e nenhuma diretiva
`//gofi:context` — o grafo enxerga as chamadas e não sabe a que contexto cada
símbolo pertence. Este modo produz **o mapa**; quem escreve a diretiva no código
é o `gofi-eng` (nunca esta skill).

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
   lista que o `gofi-eng` aplica) e, quando o dev pedir, a baseline daquele
   contexto pelo **Modo 2**. Adotar um repo não exige spec de tudo de uma vez:
   o mapa vale por si, e as baselines vêm por contexto, sob demanda.
5. **Frontmatter do contexto adotado:** `status: implementado`,
   `versao_spec: n/a` enquanto não houver spec — o código existe e é essa a
   verdade que o índice deve mostrar.
6. **Regenerar os índices** (`gofi index docs`) e devolver, nos "Próximos
   passos", que falta o `gofi-eng` gravar as diretivas.
