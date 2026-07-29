# GOFI AI

Chat com os agentes do seu projeto gofi, dentro do VSCode.

O painel conversa com um **motor de LLM plugável**. Hoje há um: o Claude,
acionado pela CLI do Claude Code rodando na raiz do workspace. É essa escolha
que faz do painel um chat *do gofi* e não um chat genérico — o processo herda o
`.claude/` do projeto, então todas as skills instaladas (`/gofi-pd`,
`/gofi-spec`, `/gofi-eng`, `/gofi-ui`, `/gofi-ops`, `/gofi-qa`, `/gofi-doc`,
`/gofi-status`, `/gofi-full`) são comandos que você digita no chat, e o agente
lê a spec, a memória e o knowledge do projeto sem nenhuma ligação extra.

## Conversas salvas

O botão de lista no canto do cabeçalho (ou `GOFI AI: conversas salvas`) abre as
conversas deste projeto — a mais recente no topo, com busca por título. Clicar
numa delas traz o histórico de volta **e retoma o contexto no motor**: a próxima
mensagem continua a conversa, não começa outra.

Não há duas noções de sessão. Cada conversa do painel é uma sessão do próprio
Claude Code (`~/.claude/projects/<projeto>/<id>.jsonl`), então:

- uma conversa começada aqui aparece no `claude --resume` do terminal — e o
  botão `›_` na linha abre um terminal já dentro dela;
- uma conversa começada no terminal aparece nesta lista, marcada `terminal`, e
  pode ser continuada no chat;
- o que for dito de um lado está no outro quando você voltar.

O `×` esquece só a cópia do painel; a conversa continua no motor. O histórico é
**local** — fica no armazenamento da extensão para este workspace e não sai da
máquina.

## Tokens e eficiência do RAG

A barra acima do chat mostra, **enquanto o agente trabalha**, quantos tokens a
conversa consumiu (entrada nova, lida do cache, gravada no cache, saída), o
custo, e um sinal do que está acontecendo com a recuperação. Abrindo a barra:

- **cada busca** (`Read`, `Grep`, `Glob`, `WebFetch`) com o alvo e quanto texto
  trouxe para o contexto, marcada `alvo` quando limitou o próprio escopo
  (`offset`/`limit`, ou um caminho) e `inteiro` quando não limitou. As buscas em
  andamento aparecem na hora, antes de terminarem;
- **o que dá para melhorar**, com o número que motivou cada apontamento.

Quando o problema é de **indexação** — um spec sem frontmatter, um doc sem
seções `## `, um `INDEX.md` que não existe — o painel não só aponta: oferece o
botão que conserta. Você confirma num diálogo que mostra a instrução exata, e o
agente aplica a correção nos arquivos do projeto como um turno normal, revisável
no diff. A extensão nunca reescreve uma spec por conta própria: quem conhece os
templates e as convenções do projeto é o agente.

## Requisitos

- VSCode 1.84+
- A CLI do [Claude Code](https://claude.com/claude-code) no `PATH` (ou o caminho
  em `gofiAI.claudeCode.executable`)
- Uma pasta aberta — o motor roda na raiz do workspace

## Onde o chat abre

Dois lugares, e você escolhe:

- **Barra lateral** — o ícone GOFI AI. Pode ser arrastado para a *Barra
  Lateral Secundária* (à direita); o VSCode lembra a posição.
- **Aba do editor** — uma página ao lado do código, com coluna centralizada.
  É o `GOFI AI: abrir o chat ao lado do editor`.

O ícone no canto superior direito da barra de título do editor abre um dos dois,
conforme `gofiAI.openLocation`. As duas superfícies mostram **a mesma conversa**:
mesma sessão, mesmo histórico, mesmo medidor.

## Uso

Digite. Duas teclas fazem o trabalho pesado:

| Tecla | O que abre |
|-------|------------|
| `/` | as skills instaladas no projeto, filtrando enquanto você digita |
| `@` | os arquivos do projeto, por busca aproximada no caminho e no nome |

↑/↓ navegam, Enter ou Tab escolhem, Esc fecha. Os chips acima do campo listam as
skills; o arquivo **em foco** no editor aparece logo acima da mensagem — clicar
insere a referência dele. Um `@caminho` na sua mensagem vira um link que abre o
arquivo.

| Comando | O que faz |
|---------|-----------|
| `GOFI AI: abrir o chat` | Foca o painel |
| `GOFI AI: nova sessão` | Guarda a conversa atual e começa outra, com contexto limpo |
| `GOFI AI: conversas salvas` | Lista as conversas do projeto para retomar uma |
| `GOFI AI: interromper a resposta` | Mata o turno em andamento |
| `GOFI AI: abrir o motor em um terminal` | Sobe a TUI interativa do motor num terminal do VSCode |
| `GOFI AI: diagnosticar a instalação` | Diz se o motor está acessível e de onde |

O chat cobre o caminho normal. O terminal é a saída de emergência para o que um
webview não hospeda: os prompts de aprovação do próprio motor, `/resume`, e
qualquer fluxo interativo que espere um terminal de verdade.

## Configurações

| Chave | Padrão | Para quê |
|-------|--------|----------|
| `gofiAI.provider` | `claude-code` | Motor de LLM |
| `gofiAI.claudeCode.executable` | `claude` | Caminho do binário |
| `gofiAI.model` | *(vazio)* | Modelo da sessão; vazio herda o `ai.model` do `.gofi.yaml` |
| `gofiAI.permissionMode` | `default` | `default`, `acceptEdits` ou `plan` |
| `gofiAI.effort` | *(vazio)* | `low` … `max` |
| `gofiAI.showThinking` | `true` | Mostra os blocos de raciocínio, recolhidos |
| `gofiAI.showToolCalls` | `true` | Mostra cada ferramenta usada |

> O painel não tem UI de aprovação. Em `permissionMode: default` o motor recusa
> o que precisaria de confirmação e explica o porquê — use `acceptEdits` para
> deixá-lo escrever, ou o terminal quando quiser aprovar caso a caso.

## Instalação

Junto com a CLI do gofi:

```sh
gofi install extensions     # instala/atualiza a extensão no VSCode
gofi install extensions --list
```

`gofi init` faz isso automaticamente ao criar o projeto.

## Outro LLM

A camada de motor é um contrato (`src/providers/types.js`): `probe` e `send`,
com um stream de eventos normalizado. Um motor novo é um módulo em
`src/providers/` mais o slug no enum `gofiAI.provider` — a UI, a sessão e os
comandos não mudam.
