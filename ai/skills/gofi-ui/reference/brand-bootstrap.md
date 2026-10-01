# Bootstrap de marca

## Bootstrap de marca — antes da primeira tela

**As cores são do projeto — não há paleta fixa nem catálogo fechado.** Se
`.gofi.yaml` não tiver `ui.brand`, **pergunte ao usuário a marca** (uma vez por
projeto): **um** valor — um preset (`blue|violet|green`) ou a **cor-semente**
(a superfície de marca, ex. `"#AAD7FF"`). Os demais papéis (`onBrand`, `action`,
`focus`, apoio) são **derivados** dela pela receita de contraste em
[expertise/ui-design/design-tokens.md](../expertise/ui-design/design-tokens.md) §"Escolher
cores com segurança" — não os pergunte um a um. Sem preferência do usuário, use
o padrão neutro do mesmo arquivo.

> `brand` é **uma string** no `.gofi.yaml`. Escrevê-lo como bloco
> (`surface:`/`onBrand:`/…) impede o config de carregar e **nenhum comando gofi
> roda no projeto**. Ajustes finos de tom vivem no tema do DS, não no config.

O agente aplica as cores do projeto pelo **mecanismo de tema do DS configurado** —
**exatamente como o manifesto do DS documenta**, sem presumir. Conforme a superfície e
o DS, isso pode ser: injetar vars/tokens (`--brand`/`--action`/… num `<ThemeProvider>`
ou nos tokens SCSS) no **web**; passar as cores a `makeTheme(brand, mode)`/
`<ThemeProvider>` no **mobile**; ou outro mecanismo que o DS defina. **Valide o
contraste ao aplicar** (receita em design-tokens §"Escolher cores com segurança"):
`onBrand` ≥ 4.5:1 sobre `surface` e `action` ≥ 4.5:1 sobre branco — ajuste o tom
dentro da cor do projeto se reprovar.

**Persistência** (as cores são do projeto, não do harness — knowledge é domínio-neutro):

```yaml
# .gofi.yaml
ui:
  framework: react      # ou react-native
  brand: "#AAD7FF"      # preset ou cor-semente — omitir = padrão neutro
```

- Gravar `brand` no bloco de UI do `.gofi.yaml` e refletir as cores pelo **mecanismo
  de tema do DS** (conforme o manifesto do DS).
- **Se o projeto configurar as duas superfícies** (web + mobile), aplicar **as mesmas
  cores nas duas** para manter paridade (cada uma pelo mecanismo do seu DS).
- Registrar a decisão de marca em `.claude/memory/project.md` (linha curta).
