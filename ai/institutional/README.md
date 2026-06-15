# Institutional — {{NOME_DO_PRODUTO}}

Conhecimento de negócio **específico deste produto/empresa** (domínio, glossário,
atores, regras, integrações, métricas, roadmap). As skills do pipeline gofi são
**genéricas e portáveis** — não carregam nada deste produto. Todo o "quem somos,
o que fazemos, como o negócio funciona" vive **aqui**.

Esta pasta é organizada como um **RAG**: o ponto de entrada é o manifesto de
retrieval **[INDEX.md](INDEX.md)** — leia-o primeiro. Ele lista os chunks
temáticos e diz **quando carregar cada um**, para usar só o relevante e
economizar tokens.

→ **Comece por [INDEX.md](INDEX.md).**

---

> **COMO USAR ESTE TEMPLATE**
>
> 1. Copie a pasta `_template/` para `ai/institutional/{seu-produto}/` (ou para a
>    raiz de `institutional/` se for o único produto).
> 2. Preencha cada chunk substituindo os `{{PLACEHOLDERS}}` pelos valores reais.
> 3. Apague as linhas iniciadas por `> [GUIA]` — são instruções para quem
>    preenche, não fazem parte do conteúdo final.
> 4. Remova as linhas/seções que não se aplicam ao seu produto; adicione chunks
>    novos seguindo o mesmo padrão e registre-os na tabela do `INDEX.md`.
