---
name: cloud-identity
description: Identidade de nuvem com gofi base/cloud (aws, oci) — credencial resolvida uma vez, identidade de pod/máquina em vez de chave
sdk: v0.8.2
keywords: [cloud, aws, oci, credentials, irsa, workload-identity, instance-principal, api-key]
---

# Identidade de nuvem — `base/cloud/aws` e `base/cloud/oci`

Referência gerada: `.claude/sdk/go/api/base-cloud-aws.md`, `base-cloud-oci.md`.

As integrações de nuvem do SDK **não** resolvem credencial cada uma do seu
jeito: todas passam por `base/cloud/<nuvem>`. Configurar identidade é,
portanto, uma decisão só — vale para bucket, fila, secrets e assinatura.

| Módulo | Consumidores no SDK |
|---|---|
| `base/cloud/aws` | `base/bucket/s3`, `base/secrets/awssm`, `msq/provider/sqs`, `netx/awssign`, `sqln/rdsauth` (recebe um `aws.Config`) |
| `base/cloud/oci` | `base/bucket/oci`, `base/secrets/ocivault`, `msq/provider/oci` |

---

## AWS — cadeia padrão primeiro

```go
import cloudaws "github.com/gofi-labs/gofi-sdk-go/base/cloud/aws"

awsCfg, err := cloudaws.Load(ctx, cloudaws.Config{}) // zero value = cadeia padrão
```

`cloudaws.Config{Region, Endpoint, AccessKeyID, SecretAccessKey, SessionToken, Profile}`
— todos opcionais. Zero value usa a cadeia AWS: env (`AWS_*`), shared config,
IRSA / EKS Pod Identity, ECS e EC2 roles. `Endpoint` serve para LocalStack,
MinIO, R2.

- `sqs.New(ctx, sqs.Config{AWS: cloudaws.Config{...}})` ou
  `sqs.NewWithConfig(awsCfg)` — o registro por env (`MESSAGING_PROVIDER=sqs`)
  usa **só** a cadeia padrão (`AWS_REGION` etc.), não `MESSAGING_*`.
- `s3.New(ctx, s3.Config{Bucket, AWS: cloudaws.Config{...}})` — por env,
  `BUCKET_S3_*` vazios caem na cadeia padrão.

## OCI — modo sempre explícito

```go
import cloudoci "github.com/gofi-labs/gofi-sdk-go/base/cloud/oci"

cfg := cloudoci.Config{AuthMode: cloudoci.AuthWorkloadIdentity, Region: region}
provider, err := cloudoci.ConfigurationProvider(cfg) // common.ConfigurationProvider do SDK OCI
```

O SDK OCI **não** autodetecta o principal: o modo é escolhido.

| `AuthMode` | Onde vale | Material |
|---|---|---|
| `AuthAPIKey` (`api_key`, default quando vazio) | qualquer lugar | `TenancyID`, `UserID`, `Fingerprint`, `PrivateKey` (PEM), `Passphrase` opcional |
| `AuthInstancePrincipal` (`instance_principal`) | compute OCI | nenhum |
| `AuthResourcePrincipal` (`resource_principal`) | Functions, Container Instances | nenhum |
| `AuthWorkloadIdentity` (`workload_identity`) | pod OKE | nenhum |

Modo desconhecido ou campo faltando → `cloudoci.ErrInvalidConfig`.

Por env, cada integração OCI tem seu prefixo: `BUCKET_OCI_*` (bucket),
`MESSAGING_OCI_*` (fila; `OCI_PRIVATE_KEY` é fallback da chave). O `bucket.Config`
carrega o modo como `bucket.OCIAuthMode` (mesmos valores).

---

## Regras

- **Em cluster, identidade do pod/máquina** (IRSA/Pod Identity; `workload_identity`
  / `instance_principal`) — **nunca** chave estática em `.env` de produção.
  Chave só em dev local, e mesmo assim via `secret://` ou `X_FILE`.
- **Montar credencial com `base/cloud/*`**, não com o SDK da nuvem direto, quando
  o projeto precisa de um cliente próprio (ex.: outro serviço AWS): mesma cadeia,
  mesmo comportamento que o resto do binário.
- **Um `aws.Config` por processo** quando possível: `cloudaws.Load` uma vez no
  composition root e reusar (ex.: `sqs.NewWithConfig(awsCfg)`).
- Import só no composition root — domínio não conhece nuvem.
