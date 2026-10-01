# Template B — Plano de Teste QA e escolha do template

### Template B — Plano de Teste QA

Para QA tester montar plano. Foco em **casos**, não em código. Toda entrada
de `errors.go` vira ≥1 caso negativo.

```markdown
# {Título} — Plano de Teste de API

Documento baseado no código real: {arquivos lidos}.


## 1. Visão geral
[O que essa API faz e por quê. Como QA enxerga o impacto no produto.]

## 2. Pré-requisitos de ambiente
- Bearer JWT válido com as claims necessárias: [listar as reais]
- [Registros prévios necessários: entidades pai / recursos referenciados]
- [Vars de ambiente do backend que afetam comportamento]
- [Feature flags relevantes, se houver]

## 3. Casos de teste por endpoint

### 3.N METHOD /v1/path

**Pré-condições:** [estado do banco antes]

#### Caso feliz
- Request: [exemplo curl ou JSON pronto]
- Resposta esperada: HTTP {N} + body [exemplo]
- Pós-condições verificáveis: [estado do banco depois — query SQL sugerida]

#### Casos de erro a cobrir
| # | Cenário | Setup | Request | Resposta esperada | Error code |
|---|---------|-------|---------|-------------------|------------|
| 1 | ... | ... | ... | HTTP {N} | `XXX_ERROR_CODE` |
[uma linha por erro do errors.go]

#### Casos de borda
- Limite mínimo / máximo de cada campo
- `nullable: null` vs ausente vs string vazia
- Idempotência (mesma request 2×)
- Path param malformado (não-UUID, caracteres especiais)
- Body truncado / JSON inválido
- Token expirado / claims faltando

#### Cenários cross-endpoint (se aplicável)
- Sequências que envolvem 2+ endpoints (criar → ler → atualizar → ler)
- Verificar consistência de retorno entre POST e GET subsequente

## 4. Regressões a monitorar
[Comportamentos que já quebraram historicamente — vir de
.claude/memory/contexts/{contexto}.md quando há histórico de QA prévia]

## 5. Variáveis a parametrizar nos testes
[O que rotacionar entre execuções: IDs, datas, valores numéricos
limítrofes, formatos regionais/locales]

## 6. Smoke test sugerido
[Sequência mínima de 3–5 requests que valida o happy path completo do
contexto, pronta para colar em Postman/Bruno/curl]
```

### Como decidir o template

| Pedido | Output |
|--------|--------|
| "documente o endpoint X" / "doc do contexto Y" | Template A (cliente) |
| "plano de teste para X" / "casos de teste do endpoint X" / "como QA testa Y" | Template B (QA) |
| "documente X para frontend e QA" / "doc completa de Y" | Ambos — dois arquivos separados |

Não misture A e B no mesmo arquivo — públicos consomem diferente.

