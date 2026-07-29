# Changelog

## Não publicado

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
- **Indicador de trabalho novo**: um bonequinho chicoteando o robô, no lugar da
  marca do gofi animada. A marca agora só identifica quem fala.
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
