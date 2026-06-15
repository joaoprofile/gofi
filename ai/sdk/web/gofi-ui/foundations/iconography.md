# Iconografia

## Estilo

- **Um único conjunto de ícones** por app (line ou duotone), com peso de traço
  consistente. Não misture famílias.
- Tamanhos padrão: 16 (inline no texto), 20 (botões/inputs), 24 (navegação),
  alinhados à escala 4/8.
- A cor herda do contexto (`currentColor`) — o ícone segue `--tx-ink` ou
  `--action`, nunca uma cor avulsa.

## Acessibilidade de ícones

| Caso | Regra |
|------|------|
| Ícone **decorativo** (ao lado de texto) | `aria-hidden="true"` |
| Ícone **informativo** sozinho (ex: status) | `role="img"` + `aria-label` |
| **Botão de ícone** (ação sem texto) | `aria-label` obrigatório com a ação |

```tsx
// botão só de ícone — label acessível obrigatório
<button aria-label="Fechar" className="icon-btn"><CloseIcon aria-hidden /></button>
```

- Um ícone informativo precisa de **3:1** de contraste (UI).
- Nunca transmita status **apenas** por ícone/cor — combine com texto.
- Alvo de toque mínimo para um botão de ícone: **44×44** (mesmo com um ícone de 20px).
