---
tipo: delta
formato: 3                        # versão do formato do corpus — não invente outro valor
contexto: {contexto}
versao: "1.0"
status: proposto                  # aprovado com a mudança, por `gofi change approve`
keywords: [{até 12 termos}]
atualizado: {YYYY-MM-DD}
---
# Delta — {contexto} — {mudanca}

> O que a mudança {mudanca} faz à spec viva de {contexto}, por ID. Cada
> requisito vem **inteiro**, como ficará — nunca "igual ao anterior, exceto…":
> é assim que o arquivamento o aplica sem interpretar. Seção sem requisito sai
> do arquivo.
>
> - **ADICIONADO**: IDs que a spec viva ainda não tem (o próximo número livre).
> - **MODIFICADO**: IDs que ela tem — o requisito e todos os seus cenários.
> - **REMOVIDO**: IDs que ela tem — só o título e o porquê.
> - **Propósito**: só quando {contexto} ainda não tem spec viva; vira o
>   Propósito dela.
> - `depende_de: [...]` no frontmatter troca o da spec viva ao arquivar.

## Propósito

{Só para contexto novo: o que ele é, o problema que resolve e onde termina. Apague a seção se a spec viva já existe.}

## ADICIONADO

### Requisito: REQ-NN — {Título do requisito}
O sistema DEVE {capacidade}.

#### Cenário: REQ-NN.1 — {Título do cenário}
- DADO {estado}
- QUANDO {ação}
- ENTÃO {resultado observável}

## MODIFICADO

### Requisito: REQ-NN — {Título do requisito}
{O requisito inteiro, como ficará, com todos os seus cenários.}

## REMOVIDO

### Requisito: REQ-NN — {Título do requisito}
**Por quê:** {motivo}
