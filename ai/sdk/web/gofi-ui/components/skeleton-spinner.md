# Skeleton · Spinner

Comunicando **loading**. Escolha pelo contexto (princípio 6, performance percebida).

| Componente | Quando |
|-----------|--------|
| **Skeleton** | carregar **conteúdo com layout conhecido** (cards, linhas, tabela) — preferido |
| **Spinner** | uma ação pontual sem layout (button `loading`, overlay curto) |

> Nunca uma **tela em branco** durante o loading.

## Skeleton
- Blocos cinza (`--sf-hover`) no **formato** do conteúdo real (mesma
  altura/raio), shimmer sutil (respeita reduced-motion).
- Quantidade ≈ ao conteúdo esperado (ex. 5 linhas de tabela).

## Spinner
- Tamanhos `sm/md/lg`, cor `--action` ou `currentColor`.
- Dentro de um botão: substitui o ícone, mantém o label.

## Props
```ts
interface SkeletonProps { width?: string|number; height?: string|number;
  radius?: 'sm'|'md'|'lg'|'pill'; lines?: number; }
interface SpinnerProps { size?: 'sm'|'md'|'lg'; label?: string; }
```

## Acessibilidade
- Região em loading: `aria-busy="true"`; skeleton `aria-hidden="true"`.
- Spinner isolado: `role="status"` + `aria-label="Carregando"`.
- Evite "flash": só mostre o loading após ~150ms (não pisque em respostas rápidas).

## Do / Don't
- ✅ Skeleton no formato do conteúdo (reduz CLS).
- ❌ Um spinner gigante centralizado para conteúdo que tem layout (use um skeleton).
- ❌ Loading sem um label acessível.

## Exemplo
```tsx
{isLoading ? <Skeleton lines={5} height={56} radius="md" /> : <List items={data} />}
```
