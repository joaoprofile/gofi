# Pagination

Navega pelas páginas de uma coleção (rodapé de uma [Table](table.md)/[List](list.md) —
ref. dashboard). Alternativa: scroll infinito / "carregar mais".

## Anatomia
`[ ‹ anterior ] [ 1 ] [ 2 ] [ … ] [ n ] [ próxima › ]` — a página atual num
círculo `--action` + texto branco. Truncamento com "…" em coleções longas.

## Quando usar (vs infinito)
| Caso | Padrão |
|------|--------|
| Dados tabulares, pular para uma página específica | Pagination |
| Feed/exploração contínua | scroll infinito + "carregar mais" acessível |

## Props
```ts
interface PaginationProps { page: number; pageCount: number; onChange: (p: number) => void;
  siblingCount?: number; }
```

## Acessibilidade
- `<nav aria-label="Paginação">` com uma lista de links/botões.
- Página atual: `aria-current="page"`. Botões anterior/próxima com `aria-label`,
  `disabled` nos limites.
- Foco gerenciado ao trocar de página; anuncie "página X de N".

## Do / Don't
- ✅ Mostre o total/intervalo ("1–10 de 240") perto do controle.
- ❌ Só ‹ › sem números quando o usuário precisa pular. ❌ Página atual só por cor.

## Exemplo
```tsx
<Pagination page={page} pageCount={pages} onChange={setPage} />
```
