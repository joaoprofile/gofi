# Pattern — Templates de página

Composições canônicas. Todas dentro do [App Shell](app-shell.md), partindo do
mobile e entregando os [4 estados](states.md).

## PageHeader (comum a todas)
`[ breadcrumb? ] [ título (h1) + subtítulo ] [ ações primárias à direita ]`.
Um `primary` por página.

## 1. List / Index (referência fiel — dashboard)
```
PageHeader: título + [+ Novo]
Toolbar: busca · filtros (chips) · [Segmented Control de visualização] · ordenação
[ Table/List ] ── célula com avatar+nome, Progress, Badge, uma ação em pill
Pagination
Seção "Recomendados/Relacionados": um grid de Cards (tag + título + descrição + ação outline)
```
Estados: skeleton de linha → empty/error/success.

## 2. Detail
```
PageHeader: breadcrumb + título da entidade + ações (editar/menu)
Resumo (cards de métrica/stat) + Tabs de seção
Conteúdo por tab; ações destrutivas com confirmação
```

## 3. Form-as-page
Ver [forms.md](forms.md): título, seções, footer com salvar/cancelar, validação no
blur/submit, estados enviando/sucesso/erro.

## 4. Dashboard
```
PageHeader + filtro de período
Grid responsivo de cards: stats, Progress, gráficos, listas de resumo
Alta densidade de informação, mas com hierarquia e espaço para respirar
```

## 5. Hero / Onboarding (cor da marca dominante)
```
Card grande da marca (--brand, --radius-xl, texto --tx-on-brand)
  título + subtítulo + lista de features (ícone + texto)
  CTA primário em pill + ação secundária
```
O equivalente web da tela de onboarding mobile — a marca preenche a superfície,
texto escuro via `--tx-on-brand` (nunca branco sobre a superfície de marca clara).

## 6. Auth (entrar / criar conta)
Centralizado, uma coluna, um painel de marca opcional ao lado (always-dark serve).
Microcopy: "Entrar", "Criar conta", "Esqueci a senha".

## Do / Don't
- ✅ Reusar PageHeader, toolbar e estados entre páginas (consistência).
- ❌ Inventar um layout por página. ❌ Pular os 4 estados em qualquer template com dados.
