---
description: Regras de negócio conhecidas — invariantes, sync, ingestão, identidade, compliance
topics: [regra, invariante, guard rail, {{REGRA_KEYWORD_1}}, {{REGRA_KEYWORD_2}}, webhook, scheduler, idempotência, chave de resolução, ciclo de vida, compliance, tenancy]
---

# Regras de negócio conhecidas — {{NOME_DO_PRODUTO}}

> Instanciação concreta neste produto dos padrões de discovery do playbook
> genérico da skill `/gofi-pd`. Aqui ficam os valores, nomes e decisões reais;
> o padrão abstrato fica descrito na skill.
>
> [GUIA] Cada seção abaixo é um **arquétipo** comum de regra de negócio. Mantenha
> as que se aplicam, preencha com os valores reais, e apague as que não fazem
> sentido para o seu produto. Adicione seções novas no mesmo formato.

## Invariantes / guard rails (sagrados)
> [GUIA] Regras que o sistema NUNCA pode violar — limites, travas, validações
> obrigatórias. São critérios de aceite implícitos de todo PRD do tema.

- {{INVARIANTE_1}}.
- {{INVARIANTE_2}}.
- {{INVARIANTE_3}}.

## Modelagem de {{ENTIDADE_COMPLEXA}} (estado físico do schema)
> [GUIA] Use quando há decisão de modelagem não-óbvia (ausência de FK, soft join,
> junction materializada por worker, ordem de ingestão não garantida...). Documente
> a decisão E o porquê, para o discovery não re-propor o que já foi descartado.
> Remova a seção se não houver modelagem digna de nota.

- {{DECISAO_DE_MODELAGEM}} — porque {{JUSTIFICATIVA}}.
- {{PREMISSA_ASSUMIDA_PELA_ARQUITETURA}}.

## {{PROCESSO_OUTBOUND}} vs {{DADO_INBOUND}} — pipelines distintos
> [GUIA] Arquétipo "ação que o sistema executa" (outbound) vs "fato que o sistema
> recebe" (inbound). Se houver um par assim no seu domínio (ex.: o processo de
> precificar vs o dado de preço), descreva os dois pipelines e a regra de
> discovery para não confundir. Remova se não se aplicar.

- **`{{PROCESSO_OUTBOUND}}`** é o **processo executado pela plataforma** (outbound).
  Pipeline: {{ETAPAS}}.
- **`{{DADO_INBOUND}}`** é o **dado factual** recebido (inbound). Pipeline: {{ETAPAS}}.
- **Discovery prático**: quando o usuário falar de "{{TERMO_AMBIGUO}}", **sempre
  perguntar** se é o pipeline outbound ou inbound — são PRDs/specs diferentes.

## Webhook reativo vs scheduler proativo
> [GUIA] Arquétipo de sincronização. Mantenha se o produto consome dados externos
> por eventos e/ou polling. Descreva quando cada mecanismo atua e o que elicitar.

- **Webhook (reativo)** descobre **novos** + atualiza quando a fonte notifica.
- **Scheduler (proativo)** refresca o que está **stale** + cobre fontes **sem webhook**.
- Quando coexistem, o scheduler **filtra por staleness** (só emite além do TTL).
- **A elicitar no PRD**: quais fontes têm webhook? Para as que não têm, qual
  mecanismo de descoberta? TTL por tipo de evento? Granularidade do scheduler?

## Capability matrix por {{FONTE_EXTERNA}}
> [GUIA] Quando o produto integra com várias fontes que diferem em capacidades,
> monte esta matriz (vira anexo do PRD). Substitua as dimensões pelas reais.
> Remova se houver uma única fonte ou nenhuma.

| Dimensão | {{FONTE_A}} | {{FONTE_B}} | … |
|---|---|---|---|
| Tem fetch por id? | {{✓/✗}} | {{✓/✗}} | … |
| Tem webhook? | {{quais}} | {{quais}} | … |
| Tem report/batch? | {{✓/✗}} | {{✓/✗}} | … |
| A dimensão existe no negócio? | {{✓/✗}} | {{✓/✗}} | … |

Célula "✗" → spec materializa como retorno de erro estável da bridge ({{NotSupported}}).

## Identidade de {{ENTIDADE}} — chave de resolução
> [GUIA] Como uma entidade é unicamente identificada, especialmente se a chave
> varia por fonte. Erro de chave gera duplicação ou perda silenciosa de dado.

- **{{FONTE_A}}**: chave = {{CAMPOS}} ({{POR_QUE}}).
- **{{FONTE_B}}**: chave = {{CAMPOS}} ({{POR_QUE}}).
- Ao vincular qualquer dado a {{ENTIDADE}}, **perguntar a chave de resolução por
  fonte** — não assumir uma chave única. Campo hidratado (resolvido na escrita)
  **não** compõe chave de idempotência; campo natural compõe.

## Ciclo de vida ({{ATIVAR/DESATIVAR}}) governa ingestão e retenção
> [GUIA] Como eventos de ciclo de vida da entidade gerenciada disparam backfill,
> sync e descarte. Remova se não houver gestão de ciclo de vida.

- **{{EVENTO_DE_ATIVACAO}}** pode **disparar backfill** do histórico ({{JANELA}}).
- **{{EVENTO_DE_DESATIVACAO}}** decide o destino do dado coletado: **manter** vs **apagar**.

## Persistência só de entidade gerenciada + sinal de upsell
> [GUIA] Padrão "só persisto o que é gerenciado; o resto descarto mas conto como
> oportunidade de upsell". Remova se o produto persiste tudo.

- Ingestão persiste **só {{ENTIDADE}} gerenciada**; o não-gerenciado é
  **descartado + contabilizado em métrica de upsell**.

## Compliance e tenancy
> [GUIA] Leis aplicáveis (LGPD, GDPR, HIPAA...), e a regra de isolamento de dados.

- **{{LEI_APLICAVEL}}** sobre {{DADOS_SENSIVEIS}}.
- Isolamento total por {{ENTIDADE_TENANT_RAIZ}} (sem exceções).

## A elicitar no refinamento
> [GUIA] Perguntas em aberto que todo PRD deste tema deve responder. Deixe aqui o
> que ainda não foi decidido para o discovery não esquecer de perguntar.

- {{PERGUNTA_EM_ABERTO_1}} (limites de frequência, SLA, janelas...).
- {{PERGUNTA_EM_ABERTO_2}} (restrições contratuais por fonte...).
