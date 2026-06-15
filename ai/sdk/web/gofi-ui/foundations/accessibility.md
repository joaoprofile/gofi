# Acessibilidade — WCAG 2.2 AA operacional

A11y é o **padrão**, não uma camada final. Todo PR demonstra estes
itens. Complementa o princípio 5 de
[ux-principles.md](../../../../knowledge/ui/ux-principles.md).

## Checklist por componente interativo

| Item | Regra |
|------|------|
| **Label** | todo input/controle tem um label associado (`<label htmlFor>` ou `aria-label`). Um placeholder **não** é um label. |
| **Foco visível** | nunca `outline:none` sem uma alternativa. Use `--focus` (2px + 2px offset). |
| **Teclado** | toda ação alcançável e operável por teclado (Tab/Shift+Tab/Enter/Space/Esc). Ordem de foco lógica. |
| **Semântica** | HTML nativo primeiro (`<button>`, `<nav>`, `<table>`). `aria-*` só quando o nativo não cobre. |
| **Contraste** | 4.5:1 texto / 3:1 UI ([color.md](color.md)). |
| **Alvo de toque** | mínimo 44×44px. |
| **Estado** | `aria-expanded`, `aria-selected`, `aria-current`, `aria-invalid`, `aria-busy` conforme apropriado. |

## Foco

```css
:where(a, button, input, select, textarea, [tabindex]):focus-visible {
  outline: 2px solid var(--focus);
  outline-offset: 2px;
}
```

`:focus-visible` (não `:focus`) — mostra o ring no teclado, não no clique do mouse.

## Padrões frequentes

- **Modal/Drawer**: o foco move para dentro ao abrir, fica **preso** enquanto
  aberto, `Esc` fecha, o foco **retorna** ao gatilho ao fechar. `role="dialog"` +
  `aria-modal="true"` + `aria-labelledby`.
- **Menu/Combobox**: navegação por setas, `aria-activedescendant`, `Esc` fecha.
- **Toast**: `role="status"` (não interrompe) ou `role="alert"` (urgente).
- **Erro de campo**: `aria-invalid="true"` + `aria-describedby` apontando para a mensagem.
- **Loading**: `aria-busy="true"` na região; skeleton com `aria-hidden`.

## Conteúdo

- Não dependa **só** da cor para significado (adicione ícone/texto).
- Respeite `prefers-reduced-motion` ([motion.md](motion.md)).
- `alt` descritivo em uma imagem informativa; `alt=""` em uma decorativa.
- Idioma no `<html lang="...">`.

## Testes

Navegue a tela **só com o teclado** e **com um leitor de tela** antes de marcar como
pronta. Use uma ferramenta de contraste, não o olho.
