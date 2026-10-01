# Adoção de repositório existente (gofi-eng)

## Adoção de repositório existente — gravar `//gofi:context` no código legado

Ativa quando o `gofi init` rodou sobre uma base que já existia: o código está
lá, o grafo está construído, e o campo `contexto` de todo símbolo vem **vazio**
— a ponte grafo→`specs/`→`memory/` não existe ainda. Esta é a tarefa que a
liga, e ela é **exclusivamente aditiva**: você acrescenta uma linha de diretiva
por pacote e **não toca em mais nada**.

**Entrada obrigatória:** o mapa pacote→contexto já confirmado pelo dev, gravado
em `.claude/memory/contexts/{contexto}.md` pelo `/gofi-spec` (Modo 3). **Sem o
mapa, pare e peça o `/gofi-spec` primeiro** — inferir a fronteira de contexto
por conta própria é justamente a decisão que não é sua.

**Procedimento:**

1. Para cada contexto do mapa, para cada pacote listado nele: escrever
   `//gofi:context {contexto}` imediatamente acima da cláusula `package`, em
   **um único** arquivo do pacote — a diretiva vale para todos os símbolos.
   Escolha o arquivo de forma **determinística e óbvia** (o que dá nome ao
   pacote, ou o primeiro em ordem alfabética), para que uma segunda passada
   encontre a diretiva onde a deixou em vez de criar uma nova.
2. Um símbolo que serve **outro** contexto leva a diretiva na própria
   declaração, sobrescrevendo a do pacote. Aplique isso só onde o mapa disser —
   é o caso raro, não o padrão.
3. **Pacote fora do mapa fica sem diretiva.** Infraestrutura compartilhada
   (`pkg/`, `internal/platform`, utilitários) não pertence a contexto nenhum, e
   inventar um para ela suja o grafo com um contexto que não tem spec nem
   memória. Liste os pacotes que ficaram de fora nos "Próximos passos".
4. **Nada além da diretiva.** Sem mover arquivo, sem renomear pacote, sem
   reordenar import, sem "já que estou aqui". Um repositório adotado é código
   que funciona; a adoção não é a hora de refatorá-lo. Se a leitura revelar algo
   que merece mudança, **reporte** — não execute.
5. Reconstruir e conferir:

   ```sh
   gofi index code
   gofi show <um símbolo de cada contexto>
   ```

   O `gofi show` tem que passar a imprimir `contexto: {contexto}` apontando para
   `specs/{contexto}/` e `.claude/memory/contexts/{contexto}.md`. Se vier vazio,
   a diretiva está no arquivo errado ou o nome divergiu do mapa.
6. Atualizar o `## Estado atual` de cada `memory/contexts/{contexto}.md` com o
   fato de que o contexto já está anotado, e o frontmatter (`atualizado`).

> **Limite a declarar, não a esconder:** hoje **só o extractor Go lê a
> diretiva**. Num pacote TypeScript/JavaScript ela é documentação para o
> humano e para o próximo agent, mas **não** aparece no grafo — ali a ponte
> ainda é o nome da pasta. Diga isso ao dev em vez de deixá-lo achar que o mapa
> ficou completo.
