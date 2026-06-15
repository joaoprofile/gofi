# Boilerplate — página + rota + providers (web)

## `app/providers.tsx`
```tsx
import 'gofi-ui/styles';
import { ThemeProvider } from 'gofi-ui';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

const queryClient = new QueryClient();

// Cores do projeto (do bloco `ui.brand` do .gofi.yaml). Omitir → padrão neutro da lib.
const brand = {
  surface: '#AAD7FF',  // superfície de marca (substitua pela do projeto)
  onBrand: '#0B2942',  // texto sobre a marca
  action:  '#1B72D8',  // affordance (botão/link/foco)
  accent:  '#444CE7',  // apoio (opcional)
};

export function Providers({ children }: { children: React.ReactNode }) {
  return (
    <ThemeProvider brand={brand}>       {/* cores arbitrárias do projeto (de ui.brand) */}
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    </ThemeProvider>
  );
}
```

## `pages/<Feature>Page.tsx`
```tsx
import { <Feature> } from '../features/{contexto}/<Feature>';

export default function <Feature>Page() {
  return (
    <main className="mx-auto max-w-screen-lg p-4 md:p-6">
      <h1 className="text-h1 text-ink">Título da página</h1>
      <<Feature> />
    </main>
  );
}
```

## `app/router.tsx` (lazy para fora do caminho crítico)
```tsx
import { lazy, Suspense } from 'react';
import { Spinner } from 'gofi-ui';

const <Feature>Page = lazy(() => import('../pages/<Feature>Page'));

export const routes = [
  { path: '/{contexto}', element: (
      <Suspense fallback={<Spinner aria-label="Carregando" />}><<Feature>Page /></Suspense>
  ) },
];
```

> `Providers` envolve a app **uma vez** (em `app/main.tsx`). A marca vem de
> `ui.brand`; o tema escuro é resolvido pelo `ThemeProvider` (preferência + toggle).
> Rota pesada (>100kb) ou fora do caminho crítico → `lazy` + `Suspense`.
</content>
