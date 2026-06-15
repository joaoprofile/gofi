# List / List Item

Uma coleção vertical. Use quando o conteúdo é mais "linha rica" do que "grade de
dados" (table) — ou como o colapso mobile da [Table](table.md).

## Anatomia (List Item)
```
[ leading: avatar/ícone ] [ body: título + subtítulo ] [ trailing: meta/ação/chevron ]
```
Altura mínima 56px, divisor `--sf-border`, hover `--sf-hover` se clicável.

## Variantes
- `default` · `interactive` (linha clicável → `<a>`/`<button>`) · `selectable`
  (checkbox/radio à esquerda).
- Densidade `comfortable` / `compact`.

## Estados
`loading (skeleton) · empty · error · success`; item: `default · hover · selected · disabled`.

## Props
```ts
interface ListItemProps { leading?: ReactNode; title: string; subtitle?: string;
  trailing?: ReactNode; href?: string; onClick?: () => void; selected?: boolean; }
```

## Acessibilidade
- Estrutura `<ul>/<li>` (ou `role="list"`). Um item clicável é um controle real.
- `aria-current` no item ativo (navegação). Seleção com um checkbox rotulado.
- Lista longa: a virtualização não pode quebrar o foco/leitor de tela.

## Do / Don't
- ✅ Título forte + subtítulo `--tx-ink-2`.
- ✅ Ação destrutiva por item via swipe/menu com confirmação.
- ❌ Uma linha totalmente clicável com sub-botões dentro (alvo ambíguo).

## Exemplo
```tsx
<ul role="list">
  {items.map(i => <ListItem key={i.id} leading={<Avatar name={i.name}/>}
     title={i.name} subtitle={i.role} trailing={<Chevron/>} href={i.url}/>)}
</ul>
```
