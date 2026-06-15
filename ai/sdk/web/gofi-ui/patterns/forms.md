# Pattern — Formulários

Formulários são onde a UX mais falha. Regras operacionais (complementam
[Field](../components/field.md) e [ux-principles.md](../../../../knowledge/ui/ux-principles.md)).

## Estrutura
- **Form-as-page** para formulários longos: título, seções com headings, ações fixas
  no footer (salvar/cancelar). Curto/pontual → um [Drawer/Modal](../components/modal-drawer.md).
- Uma coluna (mobile-first). Agrupar campos relacionados; ordem de leitura lógica.
- Campos opcionais marcados; o restante assumido como obrigatório (ou vice-versa — seja
  consistente).

## Validação
| Quando | Como |
|------|-----|
| Enquanto digita | **não** validar o primeiro caractere; no máximo feedback positivo |
| No blur | validar o campo e mostrar um erro específico |
| No submit | validar tudo, focar o **primeiro** campo inválido, um resumo se houver muitos erros |

- Erro = `aria-invalid` + uma mensagem via `aria-describedby` ([Field](../components/field.md)).
- Mensagens **específicas e acionáveis**: "Informe um e-mail válido", não "campo inválido".

## Estados do formulário
`pristine · editando · enviando (botão loading, campos read-only) · sucesso
(confirmação + próximo passo) · erro (erro de negócio inline; técnico via Banner + tentar de novo)`.

## Microcopy
| Use | Evite |
|-----|-------|
| Salvar, Entrar, Sair, Adicionar, Desfazer | Submeter, jargão |
| "Informe…", "Selecione…" | jargão técnico |

Use a linguagem do produto; mantenha verbos diretos e consistentes.

## Erros vs lapsos (princípio 3)
- **Erro consciente** → validação clara + microcopy útil.
- **Lapso inconsciente** → confirmar o destrutivo; permitir **desfazer** no reversível;
  proteger contra perda de dados ao sair com alterações pendentes.

## Acessibilidade
- Todo controle com um label visível ([Field](../components/field.md)).
- Submeter com `Enter` no formulário; `<button type="submit">`.
- Agrupar controles relacionados com `<fieldset>/<legend>` (ex.: radios).
- Não desabilitar o submit sem dizer o que está faltando (prefira validar no clique).

## Do / Don't
- ✅ Preservar dados no voltar/reload (rascunho) quando fizer sentido.
- ✅ `autocomplete`/`type` corretos (agilizam no mobile).
- ❌ Resetar o formulário inteiro por causa de um erro. ❌ Placeholder como label.
