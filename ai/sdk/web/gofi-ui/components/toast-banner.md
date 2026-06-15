# Toast · Banner

Feedback do sistema. Distinguidos por **persistência e escopo**:

| Componente | Duração | Escopo | Uso |
|-----------|---------|--------|-----|
| **Toast** | transitório (auto-dismiss) | global (canto) | confirmação de ação ("Salvo") |
| **Banner** | persistente (até resolver/fechar) | contextual (topo de uma seção/página) | aviso de sistema, erro de carga, modo offline |

## Anatomia
`[ ícone de status · mensagem · ação? · fechar? ]`. Cor por status via `--{status}` +
`--{status}-bg` ([color.md](../foundations/color.md)).

## Toast
- Posição `--z-toast`; dura ~4–6s; pausa no hover/focus; empilha no máximo ~3.
- Ação opcional (ex. **Desfazer** — preferível a confirmar antes, princípio 10).

## Banner
- Não some sozinho; oferece uma ação para resolver e/ou fechar.
- Erro de página: banner + retry.

## Props
```ts
type Tone = 'success' | 'warning' | 'danger' | 'info';
interface ToastOptions { tone: Tone; message: string; action?: { label: string; onClick: () => void }; duration?: number; }
interface BannerProps { tone: Tone; title?: string; children: ReactNode;
  action?: ReactNode; onDismiss?: () => void; }
```

## Acessibilidade
- Toast: `role="status"` (`aria-live="polite"`) para sucesso; `role="alert"`
  (`assertive`) para um erro urgente.
- Não transmita só por cor (ícone + texto). Um toast não pode ser o **único** canal
  para info crítica (ele desaparece).
- Tempo suficiente para ler; pausa no foco (WCAG "tempo ajustável").

## Do / Don't
- ✅ "Desfazer" no toast para ações reversíveis.
- ❌ Toast para um erro que exige ação (use Banner/inline). ❌ Um toast eterno.

## Exemplo
```tsx
toast({ tone: 'success', message: 'Salvo', action: { label: 'Desfazer', onClick: undo } });
<Banner tone="danger" action={<Button onClick={retry}>Tentar de novo</Button>}>
  Não foi possível carregar os dados.
</Banner>
```
