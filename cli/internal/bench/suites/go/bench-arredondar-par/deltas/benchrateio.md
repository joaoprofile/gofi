---
tipo: delta
formato: 3
contexto: benchrateio
versao: "1.0"
status: proposto
keywords: [arredondamento, par, milesimos, centavos]
---
# Delta — benchrateio — bench-arredondar-par

## Propósito

A loja recebe valores de parceiros em milésimos de real e precisa convertê-los em centavos sem viés de arredondamento.

## ADICIONADO

### Requisito: REQ-01 — Milésimos para centavos, meio para o par
O sistema DEVE receber um valor em milésimos de real (inteiro, 10 milésimos = 1 centavo) e devolvê-lo em centavos (inteiro), arredondando o que sobra ao centavo pela regra do meio para o par: abaixo da metade para baixo, acima para cima, e exatamente na metade para o centavo par.

#### Cenário: REQ-01.1 — Exatamente na metade, para baixo
- DADO um valor de 2505 milésimos
- QUANDO a conversão é feita
- ENTÃO o valor é 250 centavos

#### Cenário: REQ-01.2 — Exatamente na metade, para cima
- DADO um valor de 2515 milésimos
- QUANDO a conversão é feita
- ENTÃO o valor é 252 centavos

#### Cenário: REQ-01.3 — Acima da metade
- DADO um valor de 2506 milésimos
- QUANDO a conversão é feita
- ENTÃO o valor é 251 centavos

#### Cenário: REQ-01.4 — Sem fração
- DADO um valor de 2500 milésimos
- QUANDO a conversão é feita
- ENTÃO o valor é 250 centavos

### Requisito: REQ-02 — Valor negativo
O sistema NÃO DEVE converter um valor negativo: recusa com um erro de valor inválido.

#### Cenário: REQ-02.1 — Valor negativo
- DADO um valor de -10 milésimos
- QUANDO a conversão é feita
- ENTÃO recebe recusa por valor inválido
