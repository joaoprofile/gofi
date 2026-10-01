# Convenção de output — docs/{contexto}/

## Convenção de output — `docs/{contexto}/`

A doc é organizada **igual a PRD e spec**: uma pasta por contexto, com os
arquivos de doc dentro. Um contexto pode ter **mais de uma doc** (ex.: um
recurso por handler/agregado, ou doc separada por endpoint).

| Artefato | Caminho |
|----------|---------|
| PRD | `prd/{contexto}/prd-{contexto}.md` |
| Spec | `specs/{contexto}/sdd-{contexto}.md` |
| **Doc (esta skill)** | `docs/{contexto}/doc-{tipo}-{recurso}.md` |

Regras de nomenclatura:

- `{contexto}` — bounded context (nome real descoberto da memória/projeto).
  **Sempre** vira a pasta `docs/{contexto}/`.
- `{tipo}` — `frontend` (Template A) ou `qa` (Template B).
- `{recurso}` — discriminador da doc dentro do contexto. Use o nome do
  agregado/handler documentado em kebab-case. Se a doc cobre o **contexto
  inteiro** (todos os handlers num só arquivo), use `{recurso} = {contexto}`
  → `doc-frontend-{contexto}.md`.

Exemplo de estrutura (nomes ilustrativos):

```
docs/
├── {contexto-a}/
│   ├── doc-frontend-{recurso-1}.md   # handler do recurso 1
│   ├── doc-qa-{recurso-1}.md
│   ├── doc-frontend-{recurso-2}.md   # segunda doc do mesmo contexto
│   └── doc-qa-{recurso-2}.md
└── {contexto-b}/
    ├── doc-frontend-{contexto-b}.md  # contexto inteiro num arquivo só
    └── doc-qa-{contexto-b}.md
```

- **Nunca** escreva flat em `docs/` (ex.: `docs/frontend-{contexto}.md`) —
  sempre dentro de `docs/{contexto}/`.
- Antes de gerar, se já existe `docs/{contexto}/doc-{tipo}-{recurso}.md` com
  o mesmo recurso, **sobrescreva** (a doc é derivada do código, é a fonte da
  verdade atual do contrato). Se o recurso é diferente, **crie arquivo novo**
  ao lado — não concatene docs de recursos distintos no mesmo arquivo.

