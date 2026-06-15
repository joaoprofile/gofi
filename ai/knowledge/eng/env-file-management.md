---
name: env-file-management
description: Sempre que implementar um contexto que precisa de variáveis de ambiente, verificar/popular o `.env` na raiz do projeto e avisar o dev para preencher os valores reais
type: feedback
---

Toda vez que `gofi-eng` implementar um contexto que consome variáveis de ambiente (DB, cache, IAM, IDP, mensageria, secrets), **antes de finalizar a entrega**:

1. **Localizar o `.env` na raiz do projeto** (mesmo nível do `.gofi.yaml`).
2. **Se o arquivo não existir**, criar com as variáveis do contexto.
3. **Se existir**, ler o conteúdo e **adicionar apenas as variáveis ainda ausentes** — nunca sobrescrever valores que o dev já configurou.
4. **Usar nomes do padrão gofi** (ver `.claude/sdk/<lang>/sdk-docs/config.md` e `.claude/sdk/<lang>/knowledge/env-vars-standard.md`). Variáveis fora do padrão (IDP externo, APIs de terceiros) são exceções legítimas e devem estar documentadas na spec §0.1.
5. **Preencher com placeholders explícitos** (ex.: `change-me`, `replace-with-real-value`, `your-google-client-id`) — nunca valores reais ou plausíveis (que podem ser usados por engano).
6. **Avisar o dev no resumo final** com a lista de variáveis adicionadas e instrução clara: "preencha os valores reais antes de subir o ambiente".

**Why:** o `.env` é o canal de configuração local do dev. Esquecer de adicionar uma variável significa que o serviço sobe quebrado em desenvolvimento, e o dev descobre o gap em runtime. Adicionar com placeholders explícitos torna o gap visível antes do `docker compose up` — e o dev sabe exatamente o que precisa preencher.

**How to apply:**

- Lista a entregar no `.env` é o conjunto mencionado em §0.1 da spec ("Variáveis de ambiente adicionais") + as variáveis padrão dos módulos consumidos pelo wiring (ex.: `gofi.AddDatabase` → `DATABASE_*`, `gofi.AddCache` → `CACHE_*`, `iam.NewDefault` → `JWT_SECRET` + IDP envs).
- Se o `.env` já existir e tiver as variáveis, **não duplicar** — registrar no resumo "todas as variáveis necessárias já estão presentes".
- Variáveis sensíveis (secrets, keys, passwords) recebem placeholder genérico — o `.env` não vai pro git, mas o agent não tem como saber o que o dev quer.
- O aviso ao dev fica em "Próximos passos" no output final, junto com `Executar migration` e `Executar /gofi-qa`.

**Exceção:** se o contexto não consome nenhuma env var (raro — quase todo contexto persiste em DB), pular esse passo silenciosamente.
