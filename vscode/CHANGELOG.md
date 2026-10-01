# Changelog

## 0.3.1

- **O arquivo em foco não some quando você clica no chat.** Enquanto continua
  aberto — visível ou numa aba —, ele segue "em foco", com a seleção. E agora
  vai com a mensagem como referência: o caminho e as linhas selecionadas (não o
  conteúdo; o agente lê o que precisar). Um clique em "em foco" desliga.
- **Pergunta sobre o projeto vai com os ponteiros.** "Explique como funciona o
  pricing", "o que faz este repositório", "analise…", "descreva…" são lidos
  pelas regras — sem consultar o modelo leve — e vão à conversa com os trechos
  que respondem, para o modelo ler só aquilo em vez de varrer o repositório.
- **Cada mensagem é planejada uma vez só.** O painel avisa o hook do gofi no
  Claude Code que já planejou o texto livre; antes, o hook planejava a mesma
  mensagem de novo, num processo à parte (0,4–0,9 s a mais por mensagem).
- **Chat e grafo no mesmo painel.** Um seletor no cabeçalho — `chat | grafo` —
  troca a perspectiva sem abrir outra aba; a conversa segue viva atrás do
  grafo. O grafo em aba própria continua no comando "GOFI: grafo do projeto".
- **O grafo abre num mapa de contextos.** Em vez de centenas de documentos, um
  nó por contexto: o anel mostra do que ele é feito (PRD, spec, memória), a cor
  o estágio (implementado, com spec, só PRD, sem documento), o número no meio
  quantos documentos tem, e a linha, mais grossa quanto mais, quem cita quem ou
  divide tabelas. Duplo clique abre o contexto; `documentos` mostra o quadro
  completo. Visual novo: fundo pontilhado, legenda em cartão, rótulos em
  etiqueta, detalhes em painel flutuante, linhas no azul do gofi.
- **As linhas entre as áreas do painel são o azul do gofi**, como a moldura do
  terminal — cabeçalho, barra de uso, prompt, etiquetas, rolagem — em vez da
  borda do tema do editor.
- **As skills viram etiquetas**: `/` em ciano, o prefixo que todas dividem
  (`gofi-fw-`) apagado e o nome em destaque, com borda e realce ciano ao passar
  o mouse.
- **O nome do projeto, do `.gofi.yaml`, abre a faixa de boas-vindas**, em
  destaque, seguido de GOFI AI e da versão.
- **O planejamento leva milissegundos.** O painel mantém um `gofi intake
  --serve` aberto por projeto, com o índice carregado: de ~0,4–0,8 s por
  mensagem para ~10 ms (o primeiro pedido carrega o índice). Recarrega sozinho
  quando um documento, o índice ou o léxico muda; com um gofi antigo, sem
  `--serve`, volta à chamada de sempre.
- **Mais rápido na mensagem simples.** Um "oi" — sem verbo de tarefa, sem
  artefato, sem nada do projeto — vai direto ao motor, sem cartão de plano e
  sem consultar o modelo light (eram ~10 s). E o motor sobe enquanto o pedido
  é planejado, em vez de depois: ~2–3 s a menos na primeira mensagem, sem
  gastar token.
- **O urso do terminal abre o painel**, pixel por pixel o mesmo do `gofi chat`,
  numa faixa ciano com a versão, o convite e a pasta do projeto.
- **O modelo aparece embaixo do prompt**, como na extensão do Claude Code
  (`Opus 5.5`, `Sonnet 5.5`…); um clique abre o seletor do `/model`. Acompanha
  a fase conduzida que roda em outro nível.
- **O painel tem a cara do `gofi chat` no terminal.** Uma página só, em fonte
  monoespaçada: o que você digita é uma linha sombreada depois de `>`, a
  resposta pende de um `●`, o plano do gofi é um `●` ciano com os motivos
  recuados embaixo. Sem cartões nem avatares; o prompt tem uma régua em cima e
  embaixo, e a linha de status mostra o modelo.
- **Uma implementação que para por decisão não conversa: pergunta pelo
  cartão.** Quando a spec não cobre algo, o papel grava a pergunta e encerra;
  a resposta vira um registro curto na spec (nível standard, sem subir a
  versão) e a implementação roda de novo com a spec completa.
- **As decisões em aberto de um PRD ou de uma spec são perguntadas antes, no
  painel.** Uma fase de elicitação num nível mais barato levanta o que nem o
  pedido nem o projeto respondem; cada decisão aparece como cartão, com a
  recomendada primeiro. O PRD ou a spec é escrito de uma vez, sem entrevista
  no modelo caro.
- **Cada fase do plano roda numa sessão nova.** A fase lê o que as anteriores
  deixaram no disco — spec, memória do contexto, código — e não a conversa
  delas nem a sua: o QA não relê mais a entrevista do PRD. A sua conversa fica
  guardada e volta quando o plano termina.
- **Uma auditoria reprovada volta à implementação um nível acima.** Quando o
  `/gofi-qa` grava `status: reprovado` na memória do contexto, o plano refaz a
  fase de implementação no nível seguinte e audita de novo — uma vez; se
  reprovar outra vez, para e deixa os achados com você.
- **Cada plano conduzido fica registrado em `.gofi/runs/`**, fora do git: o
  pedido, o que o intake decidiu, cada fase com nível, modelo, tempo e custo, o
  veredito do QA e como terminou.
- **Sonnet 5.5 no /model**, na mesma ordem do `gofi init`. O `gofi init` passa a
  propor o Opus 5.5 como padrão.
- **O medidor separa o que é novo do que vem do cache.** A barra somava tudo
  como "entrada", e a conversa relida do cache a cada mensagem — cobrada a 10%
  — é a maior parte: o número chegava a milhões e parecia gasto. Agora:
  `novos · do cache · saída`.
- **O painel planeja antes de enviar.** Um pedido em texto livre passa pelo
  `gofi intake`: aparece um cartão com o plano — cada fase com o papel, o nível e
  o motivo — e, se faltar decidir algo, as opções como botões. Depois o painel
  conduz as fases no mesmo motor, uma por turno, e para depois de um PRD ou de
  uma spec para você revisar. Uma fase de nível subido roda no modelo desse nível
  e o modelo volta ao fim. `/skill`, `!comando` e mensagens com anexo vão como
  foram escritos; sem `gofi` no PATH, a mensagem segue sem plano. Configuração:
  `gofiAI.intake` e `gofiAI.gofiPath`.
- **Fable 5.1 e Opus 5.5 no /model.** A lista de modelos do seletor ganhou os
  dois lançamentos, na mesma ordem do `gofi init`: da família mais forte para a
  mais leve, e da versão mais nova para a mais antiga.
- **Todas as skills instaladas aparecem como ativas.** O painel deixava
  "apagadas" as skills fora da lista `agents:` do `.gofi.yaml`. A lista saiu da
  CLI — toda skill é instalada, e o papel é escolhido por tarefa, não por
  projeto —, então cada skill em `.claude/skills/` vira um chip normal.
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
