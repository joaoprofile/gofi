# Changelog

## Não publicado

- **Limite de sessão atingido não chicoteia mais o motor.** Quando o Claude Code
  devolve o próprio aviso de limite ("You've hit your session limit · resets…"),
  o painel parava de mostrar isso como um erro qualquer e, se havia mensagens
  na fila, disparava a próxima direto contra o mesmo limite — cada uma
  reacendendo a animação de "trabalhando" para um turno que não podia rodar.
  Agora o painel reconhece o aviso, esvazia a fila em vez de insistir nela, e
  troca a marca de trabalho pela mesma dupla do chicote — só que paradas: o
  robô esperando, e o mascote com o chicote frouxo na mão e cara de decepção
  por não poder usá-lo agora — junto com o horário de reinício, quando o
  motor informa um.
- **Buscas pelo grafo entram na conta.** `gofi graph explain` é uma busca como
  outra qualquer, e até agora sumia entre os comandos de shell — o painel só
  media `Read`/`Grep`/`Glob` e, por construção, nunca mostrava a alternativa
  barata. Agora a barra separa quantas buscas passaram pelo grafo das que foram
  abrir a árvore, cada linha traz o selo `grafo`, e quem procurou símbolo no
  `grep` com o grafo já construído no disco vê o custo disso apontado.

  Junto vem a economia, medida em vez de estimada: a resposta do grafo nomeia
  `arquivo:linha` do símbolo e de cada chamador, que são os arquivos que um
  `grep` mandaria abrir; o painel pesa esses arquivos em disco e desconta os que
  a sessão leu assim mesmo. Tudo com `stat` assíncrono e cache sobre um texto
  que já estava na memória da extensão — zero token e nada no caminho que
  desenha o painel.
- **Conversas salvas.** Cada conversa é gravada localmente e volta pelo botão de
  lista no cabeçalho — transcrição de volta na tela e contexto retomado no motor
  (`--resume`), então a próxima mensagem continua de onde parou.
- Uma conversa do painel **é** uma sessão do Claude Code: a lista junta as duas
  origens pelo id da sessão, mostra as que começaram no terminal marcadas como
  tal, e o botão `›_` abre um terminal dentro da mesma conversa. Nada é copiado
  ou exportado para ir de um lado ao outro.
- Quando o motor não tem mais a sessão (expirada, ou de outro checkout), o painel
  diz isso e roda a mensagem sem o contexto antigo em vez de falhar.
- **Blocos de código viraram cartão**: cabeçalho com a linguagem e botão que
  copia o conteúdo. Cada resposta também ganhou um botão de copiar no canto —
  copia o Markdown que o agente escreveu, não o que o renderizador fez dele.
- **Anexar arquivo pelo `+`.** O botão abre o **diálogo do sistema operacional** —
  qualquer pasta, qualquer drive da máquina onde você está sentado. É um input de
  arquivo na própria janela, e não o `showOpenDialog` do editor, porque esse roda
  onde o workspace está: numa janela remota (WSL, SSH, container) ele responde com
  o navegador de arquivos do editor, que só vê o outro sistema de arquivos. De
  quebra, os bytes chegam direto na janela e nenhum caminho precisa sobreviver à
  viagem entre dois sistemas operacionais.

  Imagem vai como imagem; arquivo de dentro do projeto vai como caminho (o agente
  lê se precisar, em vez de o prompt carregar o arquivo inteiro — é para isso que
  serve o outro item do menu, o `@`); arquivo do computador vai com o conteúdo,
  porque o motor roda na raiz do projeto e não alcança aquele caminho. O chip no
  compositor diz qual dos dois vai acontecer, e avisa quando o conteúdo foi
  truncado ou é binário. `Ctrl+V` continua colando imagem.
- **Indicador de trabalho novo**: um bonequinho chicoteando o robô, no lugar da
  marca do gofi animada. A marca agora só identifica quem fala.
- O chicote virou uma cena com os personagens do produto: o **mascote do gofi**,
  de armadura, chicoteia o **robô** que digita sem parar no teclado — com estalo,
  susto e uma gota de suor um tempo depois. E a palavra ao lado agora diz o que o
  agente está fazendo de fato (`lendo`, `editando`, `executando`, `pensando`,
  `delegando`…), voltando a girar entre palavras genéricas quando não há nada
  mais específico a dizer.
- O chicote virou o assunto do desenho: **cor de fogo** (gradiente amarelo no
  cabo, vermelho na ponta, com brilho), mais grosso, e um **movimento circular**
  de seis tempos — sai de trás do mascote, sobe atrás da cabeça (escondido pelo
  corpo, que é o que dá a impressão de volta completa), passa por cima, **estala
  no robô**, **enrola o robô inteiro** (três voltas, do pescoço ao colo, metade
  na frente e metade atrás do corpo — é isso que faz a corda parecer dar a volta)
  e volta para trás. O mascote se joga para trás no início e para a frente na
  chicotada; o robô se contorce enquanto está amarrado — e continua digitando.
- O mascote ganhou o **G no escudo**: quem está com o chicote é o gofi.
- **Enviar e parar viraram ícones** — seta e quadrado, no formato e no ciano da
  marca, no lugar de dois botões escritos.
- Correção: `userBubble` era chamado sem existir — toda mensagem do usuário
  quebrava a renderização do turno.

## 0.1.0

Primeira versão.

- Painel de chat **GOFI AI** na barra lateral, com streaming token a token,
  blocos de raciocínio recolhíveis e uma linha por ferramenta usada.
- Motor plugável (`src/providers/`); implementado o Claude via CLI do Claude
  Code, rodando na raiz do workspace — as skills `/gofi-*` do projeto viram
  comandos do chat.
- Descoberta nativa das skills do projeto: lê `.claude/skills/gofi-*/SKILL.md`
  do disco (não o `agents:`), mostra cada uma como chip com o papel no tooltip, e
  oferece autocomplete ao digitar `/` — ↑/↓ navegam, Enter ou Tab escolhem. Um
  `FileSystemWatcher` mantém a lista viva quando a CLI instala ou remove skills.
- Medidor de tokens e eficiência de recuperação em tempo real: entrada/cache/
  saída direto do motor, e o tamanho de cada `Read`/`Grep`/`Glob` conforme
  acontece.
- Auditor de RAG: abre os docs que o agente leu e verifica frontmatter,
  `keywords`, seções `## ` e a existência do `INDEX.md` do corpus. Cada
  problema vem com um botão que, após confirmação, manda o agente corrigir o
  arquivo.
- Comandos: nova sessão, interromper, abrir o motor num terminal, diagnosticar.
- Instalação pela CLI: `gofi install extensions` (e automaticamente no
  `gofi init`).
