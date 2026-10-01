# Memória e output ao concluir

## Atualização de memória ao concluir

> **Como gravar:** `gofi memory write {contexto}` com o arquivo inteiro pela entrada
> padrão — não `Edit`/`Write`. O Claude Code trata `.claude/` como sensível e pede
> aprovação a cada edição ali; numa fase conduzida ninguém aprova, e a memória não
> fecha. O comando confere o frontmatter (`contexto`, `versao`, `status`, `keywords`)
> antes de gravar. Protocolo: `.claude/expertise/harness-protocols/memory.md`.

Aplicar as três:

### 1. `.claude/memory/contexts/{contexto-infra}.md`

(`{contexto-infra}` = nome do contexto de plataforma/infra, ex.: `infra`,
`platform`.)

Pós-baseline v1.0, **não** apende entrada datada. Ao concluir:
1. **Refresh do `## Estado atual`** — recursos provisionados, ambientes cobertos, coexistência legado↔novo e decisões entram como as-built vigente.
2. **Adicione uma linha** ao `## Histórico de versões` + atualize `atualizado` no frontmatter (inclua o plan `N add/change/destroy` no resumo).

Protocolo em `.claude/expertise/harness-protocols/memory.md`.

### 2. `.claude/memory/contexts/{contexto-infra}.md` — frontmatter

```yaml
status: {estágio corrente}
atualizado: {data}
```

> O índice global é gerado por `/gofi-status`. **`project.md` só é tocado**
> se nasceu um **serviço/binário novo** (tabela "Serviços").

### 3. `specs/{infra|platform}/sdd-*.md`

- **Histórico de versões** — **nenhuma linha por fase.** A versão sobe, e o
  histórico ganha a sua linha, uma vez: quando o fluxo inteiro fecha (lei
  comum 4). A divergência entra na seção a que pertence, não num registro dela.
- **Topologia** — registrar recursos/outputs reais quando diferirem do previsto
- **Migração** — atualizar o estado do corte legado→novo (o que coexiste, o
  que já saiu)

---

## Output esperado

```
### Arquivos criados
- {ops.path}/iac/global/{...}.tf
- {ops.path}/iac/modules/{capability}/{main,variables,outputs}.tf
- {ops.path}/iac/envs/dev/{main.tf,backend.tf,dev.tfvars}
- {ops.path}/deploy/base/{service}.yaml
- {ops.path}/deploy/overlays/dev/{...}
- {ops.path}/ci/{build,test,scan}.sh
- {ops.path}/pipelines/{pipeline do provedor}.yml
- {ops.path}/README.md                              (topologia PlantUML)

### Plan (dev)
- Add: {N recursos}  Change: {N}  Destroy: {N}
- Stateful afetado: {nenhum | lista — exige aprovação}

### Coexistência com o legado
- Permanece: {mecanismo vigente intacto}
- Migra: {o que o novo passa a cobrir}
- Corte pendente: {condição declarada na spec}

### Decisões
- [ADR inline quando relevante]

### Próximos passos
- Revisar o plan e aprovar `apply` em dev
- Configurar secrets no secret store (nada versionado)
- Validar deploy em dev antes de promover staging/prod
```
