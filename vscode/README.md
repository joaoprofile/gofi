# GOFI AI

O gofi dentro do VSCode: o chat no terminal do editor, e as duas coisas que um
terminal não desenha — o grafo do projeto e a execução de uma entrega.

A conversa é o próprio `gofi chat`, o mesmo de qualquer terminal. A extensão só
o abre, empresta as teclas do editor e aponta para ele o arquivo que você está
editando; tudo o que uma conversa faz mora na CLI, uma vez só.

## Chat

O **rosto do gofi** na barra de título do editor — ou `Ctrl+Shift+P` →
**GOFI: Chat in Terminal** — abre o `gofi chat` num terminal, na raiz do
projeto. Chamar de novo mostra o terminal que já existe.

| No terminal do chat | O que faz |
|---|---|
| `Shift+Enter` | quebra a linha |
| `Ctrl+V` | cola o texto da área de transferência; sem texto nela, anexa a imagem copiada |
| `Ctrl+Alt+G`, no editor | menciona o arquivo aberto no prompt — `@caminho`, com as linhas selecionadas (`#L10-24`) |

O que o prompt aceita — o menu do `/`, o `@`, a autorização com o diff, o
complemento durante a resposta, o painel de atividade, `/audit` — está na ajuda
do chat (`/help`) e no README do gofi.

## Grafo do projeto

O **botão do grafo**, ao lado do rosto do gofi na barra de título do editor —
ou `Ctrl+Shift+P` → **GOFI: grafo do projeto** — abre, numa aba, o que o
`gofi index` indexou — os documentos **e** o código — em três visões:

- **Visão geral** — como está cada contexto: o que foi escrito (PRD, proposta,
  design, delta, spec, memória), quantos requisitos existem e quantos estão
  vivos, cenários, tabelas, arquivos de código, de quem ele depende e quem
  depende dele, na ordem que as dependências permitem. E cada árvore de código
  com o tamanho e os maiores pacotes.
- **Grafo** — o projeto inteiro como um grafo, com a árvore dele ao lado. Os
  contextos, as bibliotecas e cada árvore de código são grupos que se abrem no
  que guardam, até os documentos e os arquivos fonte; a árvore e o desenho
  mostram os mesmos nós, com os mesmos grupos abertos. As ligações são o que os
  índices sabem: **código** que chama ou usa outro código, **documentos** que
  citam outro ou dependem de outro contexto, e o código que é de um
  **contexto**. Uma ligação que entra num grupo fechado termina no grupo, com
  o peso de tudo o que ela representa — por isso o desenho abre enxuto e se
  lê, mesmo num projeto de milhares de arquivos.

  | No grafo | O que faz |
  |---|---|
  | clique | seleciona: o painel mostra o que o nó guarda e a que está ligado |
  | dois cliques | abre ou fecha um grupo; num arquivo, abre o arquivo |
  | arrastar um nó | move e fixa onde foi solto (**reorganizar** solta todos) |
  | arrastar o fundo, rolar | desloca e aproxima |
  | passar o mouse | destaca o nó e tudo a que ele está ligado |
  | filtro | acha pelo nome ou caminho, na árvore e no desenho |
  | setas, na árvore | sobem e descem; `→` abre, `←` fecha, `Enter` seleciona |

  **enquadrar** mostra tudo o que está aberto, **recolher** volta ao desenho de
  abertura, e cada tipo de ligação se esconde no seu botão.
- **Rastreio** — um contexto da esquerda para a direita: seus documentos, os
  requisitos que eles adicionam e especificam (com título, estado e cenários),
  as tabelas que tocam e os pacotes de código que as implementam. Passar o
  mouse segue uma linha de ponta a ponta.

O painel de detalhes lista, para um
documento, os requisitos, as tabelas e quem o cita; para um arquivo fonte, os
contextos, as tabelas que implementa e os símbolos, cada um abrindo na linha.

A página lê `.gofi/index/` do disco e se redesenha quando o `gofi index` o
reescreve. Nenhum modelo é chamado.

## Execução de uma entrega

`GOFI: execução de uma entrega` desenha um `gofi run`: o fluxo como grafo, com
o caminho percorrido, e a matriz de rastreio do que foi entregue — acompanhada
ao vivo enquanto a execução roda.

## Comandos

| Comando | O que faz |
|---|---|
| `GOFI: Chat in Terminal` | Abre o `gofi chat` num terminal do VSCode |
| `GOFI: mencionar o arquivo aberto no chat` | Escreve `@arquivo` no prompt do chat |
| `GOFI: grafo do projeto` | Abre o grafo (o botão ao lado do rosto do gofi) |
| `GOFI: construir o grafo de documentos` | Roda `gofi index` num terminal |
| `GOFI: execução de uma entrega (fluxo e rastreio)` | Escolhe uma execução e a desenha |
| `GOFI: diagnosticar a instalação` | Roda `gofi doctor` num terminal |

## Configurações

| Chave | Padrão | Para quê |
|---|---|---|
| `gofiAI.gofiPath` | `gofi` | Caminho do binário `gofi` |
| `gofiAI.model` | *(vazio)* | Modelo com que o chat abre; vazio usa o `ai.model` do `.gofi.yaml` |
| `gofiAI.terminal.location` | `beside` | Onde o chat abre: ao lado do código ou no painel de terminais |
| `gofiAI.graph.openOnStartup` | `false` | Abre o grafo ao abrir um projeto já indexado |

## Requisitos

- VSCode 1.84+
- O `gofi` no `PATH` (ou o caminho em `gofiAI.gofiPath`)
- Para o chat, o [Claude Code](https://claude.com/claude-code) que o `gofi chat` usa

## Instalação

Junto com a CLI do gofi:

```sh
gofi install extensions     # instala ou atualiza a extensão no VSCode
gofi install extensions --list
```

`gofi init` faz isso ao criar o projeto.
