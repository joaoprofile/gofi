# Accordion

Seções expansíveis que revelam conteúdo sob demanda — reduz a carga cognitiva
(princípio 8). Bom para FAQ, filtros agrupados, detalhes opcionais.

## Anatomia
`[ header: título · chevron ]` → painel expansível. O header é um `<button>`.

## Variantes
- `single` (um aberto por vez) · `multiple` (vários).
- Com/sem divisores; aninhável (com parcimônia).

## Estados
`collapsed · expanded · focus · disabled`. Transição de altura `duration-200`
(respeitar reduced-motion).

## Props
```ts
interface AccordionItem { id: string; title: string; content: ReactNode; }
interface AccordionProps { items: AccordionItem[]; mode?: 'single' | 'multiple';
  defaultOpen?: string[]; }
```

## Acessibilidade
- Header: `<button aria-expanded aria-controls>`; painel `role="region"
  aria-labelledby`. Chevron `aria-hidden`.
- Teclado: Enter/Space alterna; setas opcionais entre headers.

## Do / Don't
- ✅ Conteúdo genuinamente secundário/opcional dentro.
- ❌ Esconder informação crítica atrás de um accordion. ❌ Animar altura sem
  respeitar reduced-motion.

## Exemplo
```tsx
<Accordion mode="single" items={[{ id:'q1', title:'Como funciona?', content:<p>…</p> }]} />
```
