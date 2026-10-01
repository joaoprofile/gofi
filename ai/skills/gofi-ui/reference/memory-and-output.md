# Memória ao concluir e output esperado

## Atualização de memória ao concluir

> **Como gravar:** `gofi memory write {contexto}` com o arquivo inteiro pela entrada
> padrão — não `Edit`/`Write`. O Claude Code trata `.claude/` como sensível e pede
> aprovação a cada edição ali; numa fase conduzida ninguém aprova, e a memória não
> fecha. O comando confere o frontmatter (`contexto`, `versao`, `status`, `keywords`)
> antes de gravar. Protocolo: `.claude/expertise/harness-protocols/memory.md`.

**Primeiro, atualize o grafo** — a superfície é um escopo como qualquer outro, e
quem vier depois (você mesmo na próxima tela, o `/gofi-qa`) lê o mapa. Nada o
reconstrói antes do commit — o hook de pre-commit só age no commit:

```sh
gofi index code
```

É incremental — pula a árvore em que nenhum fonte mudou —, então rodar sempre é
barato. `gofi index status` diz se o grafo ficou para trás.

Depois, aplicar **todas** as três:

### 1. `.claude/memory/contexts/{contexto}.md`

Pós-baseline v1.0, **não** apende entrada datada. Ao concluir a UI:
1. **Refresh do `## Estado atual`** — telas, componentes novos no DS e decisões de UX entram como as-built (estados cobertos loading/empty/error/success; acessibilidade contraste/teclado/aria verificada).
2. **Adicione uma linha** ao `## Histórico de versões` + atualize `atualizado` no frontmatter.

Protocolo em `.claude/expertise/harness-protocols/memory.md`.

### 2. `.claude/memory/contexts/{contexto}.md` — frontmatter

Registrar o estado de UI no frontmatter do próprio contexto (sem tocar `project.md`):

```yaml
status: implementado    # ou o estágio corrente
atualizado: {data}
```

> O índice global é gerado por `/gofi-status`. **`project.md` só é tocado** se
> nasceu um **frontend/app novo** (tabela própria de Frontends, se existir).

### 3. `specs/{contexto}/sdd-{contexto}.md`

- **Histórico de versões** — **nenhuma linha por fase.** A versão sobe, e o
  histórico ganha a sua linha, uma vez: quando o fluxo inteiro fecha (lei
  comum 4). A divergência entra na seção a que pertence, não num registro dela.
- **Estrutura UI** — adicionar pages/features/components não previstos
- **Microcopy oficial** — registrar textos finais (PT-BR) usados nas telas

## Output esperado

```
### Arquivos criados
- {pathUI}/src/features/{contexto}/{Feature}.tsx
- {pathUI}/src/features/{contexto}/use{Feature}.ts
- {pathUI}/src/features/{contexto}/__tests__/{Feature}.test.tsx
- {pathUI}/src/pages/{Page}.tsx
- {pathUI}/src/components/{NewComponent}.tsx       (se entrou no DS)
- {pathUI}/src/lib/api/{contexto}.ts                (chamadas API)
- {pathUI}/src/app/router.tsx                       (rota nova)

### Jornada coberta
- Entrada: {de onde o usuário chega}
- Ação primária: {ação}
- Erros conscientes tratados: {lista}
- Deslizes mitigados: {lista}
- Caminhos alternativos: {lista}

### Decisões de UX
- [registro inline de escolhas não-óbvias]

### Próximos passos
- Validar microcopy com produto
- Executar /gofi-qa
```
