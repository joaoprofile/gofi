# Estrutura canônica do ops/

## Estrutura canônica do `ops/` (cloud-neutra)

Layout de referência para `ops.path` (ajuste nomes ao runtime/cloud da spec;
**não** crie o que a spec não pede — YAGNI):

```
{ops.path}/
  iac/                       # Infra as Code (terraform/opentofu/pulumi)
    modules/                 # módulos reutilizáveis por capability:
                             #   network, cluster|runtime, registry, db,
                             #   cache, messaging, observability, dns, secrets
    envs/
      dev/  staging/  prod/  # root por ambiente: compõe módulos + backend + tfvars
    global/                  # account/tenancy-level: identity, dns topo,
                             #   registry, backend de state (bootstrap)
  deploy/                    # CD declarativo (manifests k8s / helm / kustomize)
    base/                    # definição comum por serviço
    overlays/{dev,staging,prod}/   # diferenças por ambiente (só patches)
  ci/                        # scripts chamados pelo pipeline (lint/test/build/scan/plan)
  pipelines/                # definição de pipeline (fina — só orquestra ci/ e deploy/)
  scripts/                   # utilitários ops compartilhados (migrate, promote, rollback)
  README.md                 # convenção + diagrama PlantUML da topologia
```

Regras de organização:
- **Módulo por capability**, parametrizado; roots de ambiente só **compõem**
  módulos + `tfvars`. Sem recurso "solto" fora de módulo.
- **Tagging/labeling obrigatório** em todo recurso: `project`, `env`,
  `owner`, `managed-by=iac`, `cost-center` (ou equivalentes do cloud) — para
  rastreio, billing e ownership.
- **Naming determinístico** `{project}-{env}-{capability}` (ajuste à
  convenção do cloud) — nada de nome manual ad-hoc.
- **Build remoto + registry**: Dockerfile por serviço (consolidar de onde
  estiver, **sem deletar o legado**); imagem buildada no CI, taggeada por SHA,
  empurrada ao `ops.registry`.
- **Deploy declarativo**: deploy = aplicar manifest/helm com a imagem por SHA;
  **rollback = reaplicar a tag anterior**, nunca hotfix manual no cluster.
- **Observabilidade como código**: o stack de telemetria também é
  IaC/manifest (não setup manual no console).
- **Migrations de banco no pipeline de deploy**, em passo dedicado e gated —
  não no boot do app sem controle de concorrência/ordem.
