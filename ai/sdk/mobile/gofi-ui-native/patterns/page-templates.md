# Pattern — Templates de tela (mobile)

Composições canônicas. Toda tela usa o `Screen` (safe area), começa simples e
entrega os [4 estados](states.md).

## 1. Lista (Index)
```
Header (título + ação ➕)
[ busca + filtros (chips) ]
FlatList de ListItem (avatar, título, subtítulo, meta/chevron)
  · pull-to-refresh · paginação infinita · ListEmptyComponent
```

## 2. Detalhe
```
Header (voltar + título + ações)
ScrollView: card de resumo (marca opcional) + seções
CTA fixo no rodapé (safe area) quando há ação principal
```

## 3. Hero / Onboarding
Ver [hero-onboarding.md](hero-onboarding.md) — card de marca + feature list + CTA pílula.

## 4. Formulário
Ver [forms.md](forms.md) — Field/Input, KeyboardAvoidingView, salvar fixo.

## 5. Auth (entrar / criar conta)
Logo/marca no topo, campos, CTA pílula full, links ("esqueci a senha", "criar conta").
Teclado não cobre o botão. Microcopy: "entrar", "criar conta".

## 6. Perfil / Configurações
Lista de seções (ListItem com chevron), switches para preferências (efeito imediato),
ação "sair" ao fim com confirmação.

## Do / Don't
- ✅ Reusar Header, Screen, estados e Tab Bar entre telas (consistência).
- ❌ Inventar layout por tela. ❌ Pular os 4 estados em telas com dados.
