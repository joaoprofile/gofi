# Princípios e regras de IaC & Delivery

## Princípios invioláveis (IaC & Delivery)

Aplicam-se em **qualquer** cloud, ferramenta de IaC ou plataforma de CI/CD:

- **Plan antes de apply.** Nunca `apply`/`deploy`/`destroy` sem gerar o
  `plan`/`diff`, **mostrar ao dev e obter aprovação**. O agent produz o
  plano; quem aplica em ambiente real é decisão humana (ou pipeline gated).
- **State remoto + lock, nunca no git.** State de IaC vive em backend remoto
  com locking; o bucket/recurso de state é bootstrap à parte (`global/`).
  Arquivo de state **nunca** é commitado nem fica local em ambiente
  compartilhado.
- **Secret nunca no git.** Zero credencial/chave/token/senha em `.tf`,
  `tfvars` versionado, manifest ou YAML de pipeline. Secrets vivem no secret
  manager do cloud / variável protegida de pipeline / external-secrets;
  `tfvars` com segredo entra no `.gitignore`. App lê via env injetada em
  runtime, não de valor versionado.
- **Artefato imutável, versionado por commit.** Imagem/binário é buildado
  **uma vez** no CI, taggeado pelo **SHA do commit** (nunca só `latest`), e
  **promovido** entre ambientes sem rebuild. **Artefato compilado não vai
  para o git** — quem produz é o pipeline, não a máquina do dev.
- **Least privilege.** Identidades, roles e policies com escopo mínimo
  necessário. Nada de `*`/admin "por conveniência".
- **Paridade de ambientes.** dev/staging/prod compartilham os **mesmos
  módulos**; diferem só por `tfvars`/overlays. Sem drift estrutural entre
  ambientes.
- **Um state por ambiente** (blast radius). Backend remoto por ambiente;
  recursos account/tenancy-level (DNS de topo, registry, identity, backend de
  state) ficam em `global/`, isolados.
- **Idempotência, zero ClickOps.** Tudo declarativo e reaplicável. Mudança
  feita no console do cloud é **drift** → importar para o código ou reverter;
  nunca deixar recurso vivo fora da IaC.
- **Coexistência > substituição.** O mecanismo de deploy vigente **não é
  removido** enquanto o novo não está validado e a spec não declara o corte.
  Migração é incremental e reversível.
- **Pipeline fino, lógica versionada.** O YAML de pipeline (CI/CD) **só
  orquestra** — toda a lógica (lint, test, build, scan, `plan`, `apply`,
  deploy, rollback) vive em **scripts versionados** sob a pasta `ops/`,
  testáveis localmente. Nada de lógica enterrada no YAML do provedor.

## Regras universais (cross-cloud / cross-tool)

- **Inventário deriva da spec + do inventário de serviços** (`project.md`) —
  nunca inventar recurso que a spec não pede nem deployar binário que não
  existe no projeto.
- **Editar infra já provisionada → análise de impacto antes de fechar.**
  Mudança em recurso compartilhado (rede, security group, role, output de
  módulo consumido por outro, versão de cluster) tem contrato com **todos**
  os consumidores: rode `plan` no escopo inteiro, classifique cada mudança
  (no-op / in-place update / **replace destrutivo**), e **sinalize replace de
  recurso stateful** (db, bucket, volume) como bloqueio que exige decisão
  explícita do dev — nunca deixe um `plan` destruir estado sem aprovação.
- **Replace de recurso com estado é evento crítico.** Qualquer `plan` que
  mostre `destroy`/`replace` de banco, storage, volume ou DNS de produção
  **para**: confirme migração/backup/janela com o dev antes de prosseguir.
- **Tudo parametrizado por ambiente via `tfvars`/overlay** — nenhum valor de
  ambiente (tamanho, réplicas, domínio, cidr) hardcoded no módulo.
- **Pin de versão** em providers, módulos e imagens base — sem `latest`
  flutuante que quebra reprodutibilidade.
- **`plan` é o contrato de revisão** — toda entrega inclui o resumo do plan
  (o que cria/altera/destrói) no output, não só "apliquei".
- **Respeitar o mecanismo de deploy vigente** — o legado roda até o corte
  declarado na spec; o novo nasce ao lado. Nunca apagar Dockerfile, descritor
  ou pipeline existente sem a spec autorizar.
- **Pipeline YAML é fino; a lógica vive em script versionado** sob `ops/` —
  o YAML do provedor só chama `ops/ci/*.sh` e `ops/scripts/*`.
- **Diagramas de topologia/arquitetura em PlantUML** (regra cross-agent),
  no README do `ops/`.

> **Limites de blast radius:** dev é onde se erra. Nunca rode `apply` em
> staging/prod a partir desta skill sem `plan` aprovado **e** confirmação
> explícita do dev para aquele ambiente. Promoção é deliberada, não
> automática para produção sem gate.
