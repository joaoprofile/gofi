# Mapa do codigo — sample

Gerado por gofi graph (go, modo `fast`).
Fonte de verdade: `gofi_graph.json`. Este relatorio e o resumo navegavel dele.

> Leia este arquivo **antes** de varrer o repositorio. Ele diz onde olhar.
> Para detalhe de qualquer simbolo use `gofi graph explain <no>` em vez de abrir arquivos.

## Resumo

3 pacotes, 5 tipos, 7 funcoes, 6 metodos em 3 arquivos (80 linhas).
Grafo com 21 nos e 39 arestas, das quais 7 sao chamadas.
Modo fast: chamadas por heuristica sintatica. 5 ambiguas e 0 nao resolvidas — rode `--deep` para precisao total.
## Pacotes

Instabilidade I = saida/(entrada+saida). Perto de 0 = base estavel, muita gente depende dele. Perto de 1 = camada de borda.

| pacote | simbolos | linhas | dependem dele | ele depende de | I |
|---|---:|---:|---:|---:|---:|
| `store` | 9 | 26 | 2 | 0 | 0.00 |
| `api` | 8 | 42 | 1 | 1 | 0.50 |
| `sample` | 1 | 12 | 0 | 2 | 1.00 |

## Pontos centrais

Simbolos com maior acoplamento. Mexer aqui tem alcance largo.

- `api.par` — func, `api/api.go:19`, entrada 2 / saida 1
  par and impar are mutually recursive.
- `api.Bootstrap` — func, `api/api.go:33`, entrada 1 / saida 2
- `store.NewAudit` — func, `store/store.go:23`, entrada 1 / saida 2
- `store.Repo` — interface, `store/store.go:5`, entrada 2 / saida 0
  Repo is the data output port.
- `main` — func, `main.go:8`, entrada 0 / saida 3
- `api.NewServer` — func, `api/api.go:8`, entrada 1 / saida 1
- `api.impar` — func, `api/api.go:26`, entrada 1 / saida 1
- `store.NewMemory` — func, `store/store.go:13`, entrada 1 / saida 1
- `store.Audit` — struct, `store/store.go:21`, entrada 1 / saida 1
  Audit wraps another Repo. It also satisfies Repo.
- `api.Server` — struct, `api/api.go:6`, entrada 1 / saida 0
  Server exposes the API.
- `store.Memory` — struct, `store/store.go:11`, entrada 1 / saida 0
  Memory keeps everything in memory.

## Comunidades

Grupos de simbolos que se chamam e se referenciam entre si. Nao seguem a arvore de pastas: e o acoplamento real.

**C0 · api** — 11 simbolos em `api`, `store`, `sample`
  nucleo: `api.par`, `api.Bootstrap`, `store.NewAudit`, `store.Repo`, `main`

## Ciclos de chamada

Funcoes que se chamam em circulo (recursao mutua). Costumam ser o ponto mais dificil de refatorar.

- 2 nos: `api.impar` ↔ `api.par`

## Como consultar sem abrir arquivos

```sh
gofi graph explain <no>          # tudo sobre um simbolo: origem, vizinhos, doc
gofi graph explain <A> --to <B>  # como A alcanca B, aresta por aresta
gofi graph open                  # abre a visualizacao HTML do grafo
```

Os nomes aceitam forma curta: `NewServer`, `api.NewServer` ou o ID completo funcionam.
