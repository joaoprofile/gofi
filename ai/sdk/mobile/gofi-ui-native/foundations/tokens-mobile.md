# Tokens — forma mobile (objeto TS)

Valores de [design-tokens.md](../../../../knowledge/ui/design-tokens.md) (fonte
única). No RN a lib `gofi-ui-native` entrega os tokens como um **objeto TS tipado**,
montado por `makeTheme(brand, mode)` e exposto via `<ThemeProvider>` + `useTheme()`.

> Um componente **nunca** hardcoda cor/medida: lê de `useTheme()`. Marca e tema são
> resolvidos no provider, não no componente.

## Como a lib monta o tema

`makeTheme(brand, mode)` combina: escalas estáticas (`space`/`radius`/`motion`/
`font`/`shadow`), os neutros do modo, status, e os papéis da marca a partir das
**cores do projeto** (não de um conjunto fechado de marcas).

```ts
export type ThemeMode = 'light' | 'dark';
// as cores vêm do projeto (.gofi.yaml → ui.brand); a lib aceita cores arbitrárias
export type BrandColors = { surface: string; onBrand: string; action: string; accent?: string };

// escalas estáticas — iguais em toda marca/tema
base = {
  space:  { 0:0,1:4,2:8,3:12,4:16,5:20,6:24,8:32,10:40,12:48,16:64 },
  radius: { sm:8, md:12, lg:16, xl:24, pill:999 },
  motion: { fast:100, base:200, slow:300 },
  font:   { display:{size:34,line:42,weight:'700'}, h1:{size:28,line:36,weight:'700'},
            h2:{…}, h3:{…}, body:{size:16,line:24,weight:'400'}, bodySm:{…}, caption:{…} },
};
shadow = { sm:{boxShadow:'0 1px 2px rgba(16,24,40,.05)'},
           md:{boxShadow:'0 2px 8px rgba(16,24,40,.06)'},
           lg:{boxShadow:'0 4px 16px rgba(16,24,40,.08)'} };

const t = makeTheme(brand, 'light');   // brand = cores do projeto (ui.brand); omitido → padrão neutro
```

Chaves do objeto `Theme` retornado (o que o componente lê):

| Chave | Papel |
|-------|-------|
| `surfacePage` `surfaceCard` `surfaceHover` `surfaceSunken` `surfaceBorder` | superfícies (trocam por modo) |
| `textColor` `textSecondary` | texto padrão / apoio |
| `colorBrand` `textOnBrand` | superfície de marca + texto navy sobre ela |
| `colorAction` `colorActionHover` | ação (botão/link/foco); clareia no dark |
| `colorActionSubtle` | a ação em alpha baixo — **tint de selecionado/ativo** (tabs, chips), nunca affordance primária |
| `focusRing` | anel de foco (= `colorAction`) |
| `colorSecondary` `textOnSecondary` | **cor de apoio (mobile mantém "secondary")** — hue complementar por marca |
| `success/warning/danger/info` (+ `*Bg`, `on*`) | status |
| `space` `radius` `motion` `font` `shadow` `palette` | escalas e primitivos |

> **Assimetria com o web:** a cor de apoio é **`secondary`** no mobile e **`accent`**
> no web — mesmo papel, nome diferente. As **sombras** do mobile são mais sutis que
> as do web (quase plano) e `display` é 34 (web 36): cada superfície tem a sua forma.

## Provider e consumo

```tsx
import { ThemeProvider, useTheme, useThemeControls } from 'gofi-ui-native';

// raiz do app — defaultMode omitido segue useColorScheme() do sistema
<ThemeProvider brand={brand}>{/* brand = cores de ui.brand do .gofi.yaml; omitido → padrão neutro */}</ThemeProvider>

// dentro de um componente
const t = useTheme();
const styles = StyleSheet.create({
  card:  { backgroundColor: t.surfaceCard, borderRadius: t.radius.lg, padding: t.space[5], ...t.shadow.md },
  title: { color: t.textColor, fontSize: t.font.h3.size, lineHeight: t.font.h3.line },
  cta:   { backgroundColor: t.colorAction, borderRadius: t.radius.pill },
});

// alternar tema manualmente
const { mode, setMode, toggleMode } = useThemeControls();
```

## Regras
- Componente lê **tudo** de `useTheme()` — nunca hex/medida solta.
- `colorBrand` (ex. `#AAD7FF`) é estável nos dois temas (identidade);
  `colorAction` clareia um passo no dark (`actionDark`).
- As cores são **do projeto** (`.gofi.yaml` → `ui.brand`), aplicadas no bootstrap
  (passadas a `makeTheme`/`<ThemeProvider>`); omitir → padrão neutro — ver modelo de
  marca em [design-tokens.md](../../../../knowledge/ui/design-tokens.md).
</content>
