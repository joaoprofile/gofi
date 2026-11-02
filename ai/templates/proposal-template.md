---
tipo: mudanca
formato: 3                        # versão do formato do corpus — não invente outro valor
contextos: [{contextos}]          # todo contexto que a mudança toca — um delta para cada
versao: "1.0"
status: proposto                  # aprovado só por `gofi change approve`; em_implementacao, aceito e arquivado pelo loop
depende_de: []                    # mudanças que precisam ser aceitas antes desta
superficies: []                   # o que a mudança toca: backend | web | mobile | infra (despacha quem implementa); vazio = a única do projeto
keywords: [{até 12 termos}]
atualizado: {YYYY-MM-DD}
---
# Mudança — {mudanca}

> Uma mudança é como o contrato anda: esta proposta, um `deltas/<contexto>.md`
> por contexto tocado e o `design.md` de como. A pessoa revisa e aprova a
> mudança inteira; o loop a implementa; aceita, `gofi change archive` aplica
> os deltas nas specs vivas e a arquiva. Contexto novo é uma mudança que só
> ADICIONA. Sem código a mudar, declare `design: n/a` no frontmatter.

## Por quê

{O problema ou a oportunidade, em 2–3 frases. Cite o PRD, se houver.}

## O que muda

- {REQ-NN novo, modificado ou removido em cada contexto, em uma linha cada}

## Impacto

{O alcance no código e nos outros contextos — o que `gofi impact` e `gofi show ctx:<contexto>` mostram. Contexto que depende deste e muda de comportamento entra aqui.}

## Fora desta mudança

- {O que fica para outra, e por quê}

## Dúvidas em aberto

- [PRECISA ESCLARECER: {pergunta} — {quem decide}]
