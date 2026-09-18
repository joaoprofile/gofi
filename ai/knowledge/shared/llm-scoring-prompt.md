# Prompt de pontuação por LLM — perguntar identidade, não semelhança

Vale para todo motor que usa LLM para **pontuar candidatos contra um
referente** e ordenar o resultado: casar entidade contra catálogo externo,
deduplicar registro, ranquear resultado de busca, classificar por
proximidade. O texto do prompt é **regra de negócio**, não detalhe de
implementação — mora na spec como RN e é guardado por teste.

## A armadilha

Pedir *"estime o percentual de **similaridade** entre o referente e cada
candidato"* é a formulação intuitiva e está errada. Ela premia o candidato
que **fala sobre** o referente em vez do candidato que **é** o referente.

O caso patológico é o item acessório/derivado com título recheado de
palavras-chave: ele repete marca, modelo, versão e atributo do referente,
então é genuinamente muito similar em superfície textual. O modelo responde
corretamente a uma pergunta ruim, e o acessório barato vence o item real.

O sintoma é característico: **o ranking fica bom quando o referente é
descrito de forma vaga e piora quando você o descreve melhor**. Enriquecer o
referente com atributos aumenta a sobreposição de palavras com o derivado, que
cita esses mesmos atributos. Se acrescentar contexto piora o resultado, a
pergunta é de semelhança quando deveria ser de identidade.

## A armadilha oposta, que a correção ingênua cria

Corrigir pedindo **classificação por identidade** com regra de teto por classe
("derivado → 0", "geração diferente → no máximo 30") mata o falso positivo e
cria um defeito novo: o pool **colapsa em dois baldes**. Medição real: 14 de 20
candidatos em `0` e o resto em `95`; noutro caso, 9 empatados no topo.

Isso importa porque a saída da operação é uma **ordenação**, e ordenação com
metade do pool no mesmo valor não ordena nada — a posição dentro do balde passa
a ser a ordem de chegada, que numa ordenação estável é a ordem de coleta,
arbitrária. Você trocou um ranking errado por um ranking ausente.

Os dois requisitos valem ao mesmo tempo: **derivado no fundo E gradiente real**.

## O formato que funciona

**1. Pergunte proporção de igualdade, não semelhança nem pertinência binária.**
*"Que percentual deste candidato é igual ao referente — quanto do referente você
obteria escolhendo este candidato?"* Isso mantém a resposta numa escala, em vez
de virar rótulo.

**2. Dê a escala em faixas contíguas, cobrindo do idêntico ao sem relação.**
Faixas encostadas umas nas outras, sem buraco entre elas, cada uma ancorada numa
classe de divergência:

- topo: idêntico em todas as dimensões declaradas
- logo abaixo: mesma entidade, divergindo só em dimensão que o referente não declarou
- meio-alto: mesma entidade, variação secundária diferente
- meio: mesma família, dimensão principal diferente (inclui lote vs. unidade)
- meio-baixo: mesma marca ou categoria, outra entidade
- fundo, mas **acima de zero**: derivado/acessório *para* o referente
- zero: sem relação

Manter o derivado numa faixa baixa e **não em zero** é deliberado: ele precisa
ficar embaixo, mas ainda participar da ordenação entre derivados.

**3. Exija desempate explicitamente.** *"Candidatos que diferem entre si em
qualquer dimensão recebem percentuais diferentes; só empata quem for igualmente
igual ao referente."* Sem essa frase o modelo agrupa em valores redondos. Com
ela, empate remanescente vira informação útil — ou é duplicata real, ou é
**dimensão que você não declarou** (foi assim que uma diferença de voltagem
apareceu, ausente do cadastro e da escala).

**4. Mande usar a escala inteira.** Diga que concentrar tudo nos extremos
destrói a ordenação, que é o produto final. É redundante com o item 3 e mesmo
assim muda o resultado.

**5. Mande usar os sinais numéricos que você já envia.** Campo que viaja no
payload e não é citado na instrução é ignorado. Ordem de grandeza de divergência
num valor numérico costuma ser o discriminador mais forte disponível.

**6. Omita campo ausente, não zere.** `omitempty` em vez de `0`: um zero literal
é lido como sinal, e sinal falso é pior que sinal ausente.

## Escolha do modelo

Discriminação fina (derivado vs. real, lote vs. unidade, geração vizinha) é
onde modelos pequenos falham primeiro, e falham **silenciosamente** — devolvem
um número plausível. Compare candidatos a modelo sobre o **mesmo payload,
repetindo cada um 3×**, e olhe duas coisas:

- **discriminação** — o par que deveria separar separa?
- **estabilidade** — o mesmo payload dá o mesmo score entre execuções?
- **dispersão** — quantos valores distintos saem num pool de N? Poucos valores
  distintos é o colapso em baldes, e não aparece se você só olhar o 1º lugar.

Estabilidade não é preciosismo quando o score é **persistido e exibido**:
pontuação que oscila torna irreproduzível qualquer decisão já tomada com base
nela, e impossível auditar depois. Fixe o modelo homologado como **default no
código** (guardado por teste), não só na variável de ambiente — senão um deploy
sem a variável cai silenciosamente no modelo não homologado.

## Como testar

Duas camadas, porque nenhuma sozinha basta:

- **Unidade, sempre verde no CI** — asserta o **contrato do texto**: que o
  prompt pede proporção de igualdade, que cada faixa da escala está presente,
  que a exigência de desempate está lá, que o sinal numérico chega ao payload.
  Guarda os **dois** defeitos: a formulação por semelhança e o colapso em baldes.
- **Ao vivo, sob build tag** (`//go:build live` ou equivalente) — chama o
  provedor de verdade sobre um payload capturado do mundo real. Não roda no
  CI; roda quando se mexe no prompt ou no modelo.

Para comparar duas formulações, faça **A/B sobre o mesmo payload real**,
capturado uma vez em arquivo e reusado — muda só a instrução. Payload
sintético não reproduz o problema: o derivado recheado de palavras-chave é
exatamente o que ninguém inventa ao escrever um teste à mão.
