# Avatar

Representa uma pessoa/entidade. Imagem, iniciais (fallback) ou ícone.

## Anatomia
Círculo (`--radius-pill`), tamanhos `xs 24 · sm 32 · md 40 · lg 48 · xl 64`. Fallback:
iniciais sobre uma cor derivada do nome (determinística) ou um ícone neutro.

## Variantes
- `image` · `initials` · `icon`.
- **Stack (+N)**: grupo sobreposto com um contador ao final (ref. dashboard — pessoas
  na tabela). Sobreposição negativa, borda `--sf-card` para separar.
- Indicador de status opcional (ponto online/offline) no canto.

## Props
```ts
interface AvatarProps { src?: string; name: string; size?: 'xs'|'sm'|'md'|'lg'|'xl'; status?: 'online'|'offline'; }
interface AvatarStackProps { items: AvatarProps[]; max?: number; } // o excedente vira "+N"
```

## Acessibilidade
- `alt` = o nome da pessoa (imagem informativa). Iniciais: `aria-label` com o nome.
- Stack: `aria-label="{n} pessoas"`; o "+N" tem um label "mais {k}".
- Status via cor **+** `aria-label` (não só cor).

## Do / Don't
- ✅ Fallback de iniciais quando a imagem falha (sem caixa de imagem quebrada).
- ❌ Avatar como único identificador clicável sem um label.

## Exemplo
```tsx
<AvatarStack max={3} items={people.map(p => ({ src: p.photo, name: p.name }))} />
```
