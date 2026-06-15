# Princípios de UX (cross-framework)

Princípios operacionais que toda tela deve atravessar antes de ser dada como
pronta. Duas camadas:

- **Fundamentos (1–5)** — o piso de qualquer interface. Adaptados de
  [Programaria — 5 dicas de UX para quem não é
  designer](https://www.programaria.org/5-dicas-de-ux-para-quem-nao-e-designer/).
- **Princípios modernos (6–14)** — o que separa um app de produção de um
  protótipo em SaaS, plataformas e apps atuais. Ancorados nas [10 heurísticas
  de Nielsen](https://www.nngroup.com/articles/ten-usability-heuristics/),
  nas [Laws of UX](https://lawsofux.com/), no [WCAG 2.2
  AA](https://www.w3.org/TR/WCAG22/) e nos [Core Web
  Vitals](https://web.dev/articles/vitals).

Estes princípios são **inegociáveis** — não há "exceção por prazo". Se um
deles não puder ser atendido, registre como dívida explícita no
`.claude/memory/contexts/{contexto}.md` com prazo de resolução.

Tema e dark mode têm doc própria:
[knowledge/ui/theming-dark-mode.md](theming-dark-mode.md).

---

# Fundamentos (1–5)

## 1. Empatia > simpatia

**O que é.** Empatia é se colocar no lugar de quem usa, entendendo dor real
e contexto. Simpatia é projetar pelo seu próprio gosto ou pela marca.

**Por quê.** Decisões por simpatia produzem soluções que parecem bonitas
para o time mas falham para o usuário. Empatia força a sair da bolha.

**Como aplicar em código.**
- Toda feature começa com 1-3 linhas de contexto no topo do arquivo da
  page ou feature: *"quem usa, em qual contexto, com qual dor"*. Quando
  óbvio, dispensa.
- Antes de escrever um componente, pergunte: "essa pessoa está com pressa,
  estressada, com tela pequena, sem internet boa?" — se a resposta é "sim"
  para qualquer item, o componente precisa cobrir esse caso.
- Personas e antipersonas (quem **não** é o usuário) ajudam a recusar
  features que servem só ao time interno.

**Sinal de violação.** "Achei mais bonito assim" sem dado/teste por trás.

---

## 2. Jornada do usuário, ponta a ponta

**O que é.** Mapear todas as etapas que o usuário percorre — entrada,
ações, decisões, saídas — identificando obstáculos (pain points) e
oferecendo caminhos alternativos.

**Por quê.** Tela isolada é miragem. O usuário vive um fluxo, e o ponto de
fricção quase nunca está na tela que você está implementando — está na
*transição* entre telas.

**Como aplicar em código.**
- Antes de qualquer componente, escreva a jornada em texto:
  ```
  Entrada → Ação primária → Sucesso
                          → Erro de negócio (com caminho alternativo)
                          → Erro técnico (com retry)
                          → Vazio inicial (com CTA de onboarding)
  ```
- Cada pain point identificado **precisa de um caminho alternativo**
  implementado, não documentado para depois.
- Em projetos grandes: feature nova quebra fluxo existente? Mapeie o
  impacto antes de mergir.

**Sinal de violação.** Tela só com caminho feliz; estado vazio sem CTA;
erro sem ação de recuperação.

---

## 3. Identificar erros e deslizes

**O que é.** Distinção de Donald Norman:
- **Erro (mistake)** — consciente. O usuário entendeu a tela errado, ou a
  tela não combina com sua expectativa.
- **Deslize (slip)** — inconsciente. O usuário sabia o certo mas clicou
  errado por desatenção.

**Por quê.** As mitigações são diferentes — tratar deslize como erro frustra
quem sabe o que está fazendo; tratar erro como deslize esconde problemas de
design.

**Como aplicar em código.**

| Tipo | Mitigação no código |
|------|--------------------|
| Erro consciente | Validação inline com microcopy explicando o que está errado e como corrigir; placeholders bem escritos; affordances visuais claras (botão parece botão). |
| Deslize | Confirmação para ações destrutivas (delete, cancelar pedido); undo (snackbar com "desfazer" por 5-10s) para reversíveis; debounce em ações que podem ser disparadas duas vezes; estado disabled durante submit. |

- Confirmação modal ≠ undo. **Modal de confirmação** é fricção
  preventiva (ações destrutivas). **Undo** é remédio pós-ação (reversíveis).
  Escolha um, não ambos.
- Microcopy de erro explica o problema e a solução: "Email já cadastrado.
  Tente entrar ou recupere a senha." — não "Erro 409".
- **Prevenir > corrigir** (Nielsen #5): desabilite o que não se aplica,
  restrinja a entrada no formato certo (date picker em vez de texto livre),
  e prefira *constraints* a mensagens de erro depois do fato.

**Sinal de violação.** Delete sem confirmação. Submit duplicado por clique
duplo. Erro mostrando código HTTP cru.

---

## 4. Mobile-first (e responsivo de verdade)

**O que é.** Projetar e codar primeiro para a tela menor; expandir para
telas maiores como refinamento.

**Por quê.** Maioria do tráfego brasileiro é mobile (68%+ exclusivos
mobile, segundo dado citado pela Programaria). Começar pelo desktop é
otimizar para a minoria. Em projetos grandes, "mobile depois" vira
"mobile nunca".

**Como aplicar em código.**
- Comece o CSS / Tailwind sem media queries (= mobile). Adicione `md:`,
  `lg:` apenas para refinamento de tela grande.
- Prefira layout **fluido** a breakpoints rígidos: `clamp()` para tipografia
  e espaçamento, grid com `minmax()`/`auto-fit`, e **container queries**
  (`@container`) quando o componente precisa reagir ao espaço do *pai*, não
  da viewport — o padrão moderno para componentes reusáveis em layouts
  variáveis.
- Touch targets ≥ 44×44px (WCAG 2.2 *Target Size*). Botões empilhados
  verticalmente em telas estreitas. Modal vira bottom sheet em mobile.
- Respeite áreas seguras (`env(safe-area-inset-*)`) e o teclado virtual
  (`100dvh`/`100svh` em vez de `100vh`).
- Teste em tela pequena **antes** de marcar PR como pronto. DevTools
  responsivo iPhone SE (375px) é o piso.

**Sinal de violação.** Tela quebrada abaixo de 768px. Botão de 32px de
altura. Tabela horizontal sem scroll ou alternativa em mobile. `100vh`
cortando conteúdo atrás do teclado.

---

## 5. Acessibilidade e branding juntos

**O que é.** Acessibilidade não é trade-off com identidade visual — é
restrição que melhora design para todo mundo.

**Por quê.** Milhões de brasileiros têm deficiências visuais, motoras,
cognitivas. Acessibilidade é também SEO, performance e clareza para o
usuário sem deficiência. WCAG 2.2 AA é o piso legal e de qualidade.

**Como aplicar em código.**

| Prática | Como verificar |
|---------|----------------|
| Contraste mínimo 4.5:1 (texto normal) e 3:1 (texto grande / UI) | Usar plugin/ferramenta (axe, Lighthouse), não olhar no olho |
| No máximo 2 typefaces no projeto | Auditar `tokens.md` do DS |
| Microcopy em PT-BR | Glossário no DS: "entrar"/"sair"/"salvar", nunca "logar"/"deslogar"/"submit" |
| Foco visível em **todo** elemento focável | Tab pelo teclado e ver onde está |
| `<label>` em todo input | Erro de teste se faltar (Testing Library `getByLabelText`) |
| Texto alternativo em imagem informativa; `alt=""` em decorativa | Code review |
| Hierarquia semântica (`h1` → `h2` → `h3` sem pular) | Lighthouse |
| HTML semântico antes de ARIA (`<button>`, `<nav>`, `<dialog>`) | Code review — *no ARIA is better than bad ARIA* |
| `prefers-reduced-motion` respeitado | Animações > 200ms desligam quando preference ativa |
| Cor nunca é único portador de informação | "Erro" sempre tem ícone + texto, não só vermelho |
| Target size ≥ 24×24px (WCAG 2.2) | Medir alvos clicáveis pequenos |

**Sinal de violação.** `outline: none` sem `:focus-visible` alternativo.
Botão "Excluir" só identificado pela cor vermelha. Texto cinza claro
sobre fundo branco. "Logar" em qualquer lugar do produto.

---

# Princípios modernos (6–14)

A diferença entre um app que *funciona* e um que as pessoas *escolhem usar*
está aqui. Para SaaS, plataformas e apps atuais, estes deixam de ser
"polish" e viram requisito.

## 6. Performance percebida é parte da UX

**O que é.** O que importa não é só o tempo real — é o tempo *sentido*. Uma
tela que responde na hora (mesmo que o servidor ainda não respondeu) parece
mais rápida que uma que congela esperando.

**Por quê.** Lentidão é o defeito de UX nº 1 em escala. Os Core Web Vitals
(**LCP** < 2.5s, **INP** < 200ms, **CLS** < 0.1) são proxy direto de
satisfação e retenção — e, na web, de ranqueamento.

**Como aplicar em código.**
- **UI otimista** em mutações reversíveis: atualize o estado local na hora,
  reconcilie quando a resposta chega, reverta com toast se falhar (TanStack
  Query `onMutate`/`onError`/rollback).
- **Skeletons** que espelham o layout final (nunca tela em branco nem
  spinner solitário no meio do vazio). Reserve o espaço para evitar *layout
  shift* (CLS).
- **Streaming / carregamento progressivo**: mostre o que já chegou em vez de
  esperar tudo (React Suspense, dados paginados, `content-visibility`).
- **Code-split** por rota e lazy em tudo fora do caminho crítico; imagens
  com `loading="lazy"`, dimensões explícitas e formato moderno.
- **Debounce/throttle** em busca-enquanto-digita; cancele requisições
  obsoletas.

**Sinal de violação.** Botão que "trava" 800ms sem feedback após o clique.
Spinner de tela cheia para algo que poderia ser otimista. Layout pulando
quando a imagem/fonte carrega.

---

## 7. Feedback contínuo: o sistema nunca fica mudo

**O que é.** Toda ação do usuário tem resposta visível e imediata — a 1ª
heurística de Nielsen (*visibility of system status*).

**Por quê.** Silêncio gera dúvida ("cliquei? travou?"), que gera cliques
repetidos e desconfiança. Feedback é o que torna a interface *conversável*.

**Como aplicar em código.**
- **< 100ms**: resposta perceptualmente instantânea (hover, press, foco) —
  sempre via estado visual, não só cursor.
- **100ms–1s**: indicador inline no próprio controle (botão vira loading,
  campo mostra spinner) — não tire o contexto do usuário.
- **> 1s**: progresso explícito (barra, contador, "processando 3 de 10").
- **Toasts** para resultado de ação fora do fluxo (salvou, falhou, copiou);
  efêmeros, com ação quando cabível ("desfazer", "ver"). Erros importantes
  não somem sozinhos.
- **Micro-interações com propósito**: a animação confirma o que aconteceu
  (item entra na lista, check anima ao salvar), 150–300ms, easing natural,
  e **sempre** sob `prefers-reduced-motion`. Animação que não comunica nada
  é ruído.

**Sinal de violação.** Ação sem nenhuma resposta visual. Toast de sucesso
que some antes de ler. Spinner global para uma ação local. Animação só
"porque fica legal".

---

## 8. Carga cognitiva mínima

**O que é.** Cada elemento na tela cobra atenção. O bom design **remove**
até sobrar só o necessário para a próxima decisão (Hick: mais opções = mais
tempo de decisão; Miller: memória de trabalho é curta).

**Por quê.** Telas densas de tudo-ao-mesmo-tempo paralisam. A interface deve
guiar o olho para a ação principal, não competir com ela.

**Como aplicar em código.**
- **Uma ação primária por tela/seção** — visualmente dominante; o resto é
  secundário/terciário (hierarquia de botões clara no DS).
- **Progressive disclosure**: esconda o avançado atrás de "mais opções",
  accordions, *wizards*. Mostre o caminho comum primeiro.
- **Defaults inteligentes**: pré-preencha o provável, lembre a última
  escolha, não peça o que dá para inferir. O melhor formulário é o que não
  precisa ser preenchido.
- **Reconhecer > lembrar** (Nielsen #6): mostre opções e contexto em vez de
  exigir que o usuário decore (breadcrumbs, valores atuais visíveis,
  autocomplete).
- **Lei de Jakob**: siga convenções consolidadas (carrinho no topo-direita,
  logo leva à home, ⚙ = configurações). Originalidade sem motivo é custo.

**Sinal de violação.** 5 botões com o mesmo peso visual. Formulário pedindo
dado que o sistema já tem. Padrão "criativo" que o usuário precisa aprender
sem ganho real.

---

## 9. Consistência via design system & tokens

**O que é.** Mesma intenção → mesmo componente, mesmo token, mesmo
comportamento, em todo o produto. Nada de reinventar botão/spacing/cor por
tela.

**Por quê.** Consistência é previsibilidade — o usuário aprende uma vez e
reusa em todo lugar (Nielsen #4). Para o time, é velocidade e menos bug.

**Como aplicar em código.**
- **Tokens, nunca literais** para cor, espaçamento, raio, tipografia,
  sombra, z-index e duração de animação. Cor especificamente:
  [theming-dark-mode.md](theming-dark-mode.md).
- **Componente do DS antes de criar um novo.** Variação nova vira *variant*
  do componente existente, não um clone.
- **Escala de espaçamento** fixa (4/8px) — sem `margin: 13px` avulso.
- **Estados padronizados** no DS: hover, focus-visible, active, disabled,
  loading, error — todo componente interativo os tem, iguais em todo lugar.

**Sinal de violação.** Três tons de azul "primário" diferentes. Botão
construído à mão numa tela porque "era mais rápido". `padding` em pixel
mágico fora da escala.

---

## 10. Perdão e estado durável

**O que é.** O usuário pode errar, fechar a aba, perder a conexão — e não
perder trabalho nem ficar preso. Controle e liberdade (Nielsen #3).

**Por quê.** Medo de perder dado ou de "não ter volta" trava o uso. Perdão
gera confiança para explorar.

**Como aplicar em código.**
- **Undo > confirmação** sempre que a ação for reversível (snackbar
  "desfazer" 5–10s). Confirmação só para o irreversível de verdade.
- **Autosave / rascunho** em formulários longos; persista localmente
  (`localStorage`/IndexedDB) e restaure ao voltar. "Você tem alterações não
  salvas" antes de sair.
- **URL como estado**: filtros, aba, paginação e busca na query string —
  recarregar, compartilhar link e voltar/avançar do navegador funcionam.
- **Resiliência de rede**: retry com backoff, fila/otimismo offline quando
  fizer sentido, e mensagem clara de "sem conexão" em vez de erro cru.
- **Saídas de emergência**: dá para cancelar processo longo, fechar modal
  com Esc/fora, sair do fluxo sem becos.

**Sinal de violação.** Refresh perde o formulário inteiro. Filtro que some
ao recarregar e não dá para linkar. Modal sem como fechar a não ser
concluindo. Delete definitivo sem undo nem confirmação.

---

## 11. Fluidez para power users (teclado & command palette)

**O que é.** Iniciante e expert usam a mesma UI por caminhos diferentes —
"aceleradores" para quem usa todo dia (Nielsen #7, *flexibility &
efficiency*).

**Por quê.** Em ferramentas de trabalho/SaaS, o uso repetido domina.
Obrigar o expert a percorrer o mesmo fluxo lento do iniciante mata
produtividade — e é o que diferencia produtos "amados".

**Como aplicar em código.**
- **Command palette** (`⌘/Ctrl+K`) para navegar e executar ações por busca —
  padrão de SaaS moderno. Centraliza atalhos descobríveis.
- **Atalhos de teclado** nas ações frequentes, com dica visível (`tooltip`
  mostrando a tecla) — atalho que ninguém descobre não existe.
- **Navegação por teclado completa**: tab order lógico, Enter/Esc
  esperados, setas em listas/menus, foco gerenciado em modais (focus trap +
  retorno ao gatilho).
- **Ações em lote** (multi-seleção) e operações repetíveis em tabelas/listas
  para quem opera em volume.

**Sinal de violação.** App de uso diário 100% dependente de mouse. Modal
que não fecha no Esc. Tabela que só deixa agir item a item. Atalho que
existe mas não aparece em lugar nenhum.

---

## 12. Onboarding e time-to-value

**O que é.** A velocidade até o usuário ter o **primeiro valor real** — não
o primeiro clique, o primeiro "aha". Estados vazios são onde o produto
ensina, não onde ele falha.

**Por quê.** A maior parte do abandono é nos primeiros minutos. Tela vazia
sem direção é fim de jornada silencioso.

**Como aplicar em código.**
- **Empty states que ativam**: ilustração + 1 frase do que vai aparecer ali
  + **CTA** da ação principal. Diferencie "vazio inicial" (nunca houve
  dado → onboarding) de "vazio de filtro" (limpar filtro) e "vazio de erro".
- **Dados de exemplo / templates** para o usuário ver a forma do resultado
  antes de produzir o seu.
- **Onboarding contextual** (tooltip/coachmark no ponto de uso) em vez de
  tour-modal gigante no início. Progressivo, pulável, não-repetível.
- **Checklist de ativação** / progresso quando o setup tem várias etapas;
  mostre o quanto falta.

**Sinal de violação.** Tabela vazia dizendo só "Nenhum dado". Tour de 8
passos antes de deixar o usuário tocar em nada. Mesmo empty state para
"nunca criou" e "filtro não achou".

---

## 13. Confiança, privacidade e segurança por design

**O que é.** A interface comunica o que está acontecendo com os dados e a
conta do usuário, sem pegadinha. Confiança é UX.

**Por quê.** Produto sério é produto em que se confia. Padrões obscuros
("dark patterns") ganham no curto prazo e perdem retenção e reputação — e
hoje custam multa (LGPD/GDPR).

**Como aplicar em código.**
- **Sem dark patterns**: opt-in claro, cancelar tão fácil quanto assinar,
  nada de pré-marcado enganoso ou "confirmshaming".
- **Ações sensíveis transparentes**: diga o que será compartilhado/excluído
  *antes*; permissões pedidas no momento do uso, com motivo.
- **Estado de sessão e segurança visível**: quem está logado, expiração de
  sessão tratada com graça (re-auth sem perder contexto), feedback de
  permissão negada ("você não tem acesso a X" — não tela branca).
- **Privacidade por padrão**: minimize dados pedidos; mascare PII na UI
  quando não precisa estar exposta.

**Sinal de violação.** Botão "cancelar assinatura" escondido em 5 cliques.
Permissão de localização pedida no load sem contexto. 403 virando tela em
branco. Checkbox de marketing pré-marcado.

---

## 14. UX de IA (AI-native) — quando o produto usa IA

**O que é.** Features de IA generativa têm UX própria: saída probabilística,
não-determinística e às vezes errada. A interface precisa deixar isso
gerenciável, não esconder.

**Por quê.** Tratar IA como botão mágico que "sempre acerta" quebra a
confiança no primeiro erro confiante. O design define se a IA *empodera* ou
*engana*.

**Como aplicar em código.**
- **Streaming da resposta** (token a token) — feedback de progresso e
  permite interromper. Nunca um spinner mudo por 20s.
- **Transparência e fontes**: mostre de onde veio (citações/links), deixe
  claro que é gerado por IA, e exiba confiança/limites quando relevante.
- **Human-in-the-loop**: a IA *propõe*, o humano *confirma* antes de ações
  com efeito (enviar, deletar, gastar). Sempre editável antes de aplicar.
- **Recuperação graciosa**: "regenerar", "tente de novo", feedback 👍/👎, e
  caminho manual quando a IA não resolve — nunca um beco sem saída.
- **Expectativa calibrada**: prompts de exemplo, limites visíveis, e
  linguagem que não promete onisciência. Erro de IA tratado como erro de
  produto, não do usuário.

**Sinal de violação.** Resposta de IA que aparece toda de uma vez após
espera longa. Saída apresentada como verdade absoluta, sem fonte nem como
contestar. IA executando ação irreversível sem confirmação.

---

# Aplicando em projetos grandes

Quando o produto cresce, esses princípios viram **infraestrutura**, não
checklist manual:

- **Empatia** vira pesquisa contínua + analytics qualitativo (PostHog /
  Hotjar session replay).
- **Jornada** vira event tracking estruturado e funis observados.
- **Erros vs deslizes** vira convenção do DS — todo botão destrutivo é
  uma variante específica que já vem com confirmação/undo embutidos.
- **Mobile-first / responsivo** vira lint + CI (testes Playwright em
  viewport mobile obrigatórios).
- **Acessibilidade** vira teste automatizado (axe-core no CI) + audit
  manual periódico (teclado + leitor de tela).
- **Performance** vira orçamento (*performance budget*) com Core Web Vitals
  medidos em campo (RUM) e travados no CI (Lighthouse CI).
- **Consistência** vira o design system como fonte única — tokens
  versionados, Storybook, e lint que barra cor/spacing fora de token.
- **Feedback / estado / IA** viram componentes do DS (toast, optimistic
  mutation hook, command palette, streaming view) — não reimplementados por
  tela.

Quando você (gofi-ui) implementa, adicione o componente/teste/convenção
que torna o princípio mecânico — não dependa de lembrar.
