# Motion

Durações/easing em [design-tokens.md](../../../../knowledge/ui/design-tokens.md).
O movimento serve a **feedback e continuidade**, nunca à decoração.

## Durações

O web **não** define vars de duração — use os utilitários do Tailwind:

| Utilitário | ~valor | Uso |
|------------|--------|-----|
| `duration-100` | 100ms | hover, tap, mudança de estado de controle |
| `duration-200` | 200ms | transição padrão, fade, expandir/recolher |
| `duration-300` | 300ms | entrada de overlay/drawer/sheet |

Easing padrão: utilitário `ease-standard` (var `--ease-standard`, definida no
`@theme`). A entrada (aparecer) pode ser ligeiramente mais lenta que a saída.

## `prefers-reduced-motion` (obrigatório)

Animação > 200ms respeita a preferência — vira um fade curto ou uma troca instantânea.

```css
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    animation-duration: .01ms !important;
    transition-duration: .01ms !important;
  }
}
```

## Boas práticas

- Anime `transform`/`opacity` (compositados), não `width`/`top`/`left` (reflow).
- Feedback perceptivamente instantâneo: resposta visual < 100ms ao tap.
- Skeleton/spinner contextual no carregamento — nunca uma tela em branco
  ([patterns/states.md](../patterns/states.md)).
- Sem movimento autônomo/infinito que distraia (carrosséis que se movem sozinhos).
