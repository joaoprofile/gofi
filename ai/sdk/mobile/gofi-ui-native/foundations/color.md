# Cor — mobile

Mesma semântica do web ([web/color.md](../../../web/gofi-ui/foundations/color.md)
e [design-tokens.md](../../../../knowledge/ui/design-tokens.md)). Aqui, o essencial RN.

## Papel duplo da cor de marca (igual ao web)
| Papel | Token | Onde | Texto |
|-------|-------|------|-------|
| Marca | `colorBrand` (ex. `#AAD7FF`) | card de marca, hero, blocos | `textOnBrand` navy |
| Ação | `colorAction` (ex. `#1B72D8`) | botão primário, link, ativo | branco |

> Superfície de marca clara → texto on-brand escuro (nunca branco); `colorAction` é
> o tom que passa AA sobre branco. **Nunca** texto branco sobre a superfície de marca
> clara — use `textOnBrand`. As cores vêm do projeto (`ui.brand`); os hex acima são só
> o exemplo de partida.

## Contraste (WCAG 2.2 AA)
4.5:1 texto / 3:1 UI. Não dependa só de cor: status = ícone + texto + cor.

## Dark mode
`useColorScheme()` define o tema. Marca estável; ação clareia um passo no escuro.
Sombra perde força no dark → reforce separação com `surfaceBorder`.

## Status
`success/warning/danger/info` com tint de fundo claro + texto escuro (warning é
claro → texto escuro). Mesmos hex da fonte única.
