# Workflow de implementação da UI

## Workflow

```
1. Ler spec → identificar telas, ações do usuário, contratos de API
2. Mapear jornada (texto curto na descrição da feature):
   - Entrada (de onde o usuário chega?)
   - Ações primária e secundárias
   - Pain points e caminhos alternativos
   - Saídas: sucesso, erro de negócio, erro técnico, vazio
3. Wireframe textual da tela (hierarquia visual, ordem de leitura mobile)
4. Identificar componentes:
   - Reutilizáveis → vão em components/ (DS interno)
   - Específicos da feature → vão em features/{contexto}/
5. Implementar de baixo para cima:
   a. Tokens / estilos compartilhados se faltarem
   b. Components atômicos (Button, Input, Field, etc.)
   c. Components compostos (Form, DataTable, etc.)
   d. Feature(s) — composição + estado local + chamadas de I/O
   e. Page(s) — composição de features + layout + título
   f. Rota(s) (lazy quando >100kb ou fora do caminho crítico)
6. Estados obrigatórios em TODA tela com dados:
   - loading (skeleton ou spinner contextual, nunca tela em branco)
   - empty (ilustração + microcopy + CTA quando aplicável)
   - error (mensagem útil + ação de retry quando faz sentido)
   - success (estado normal com dados)
7. Acessibilidade obrigatória em TODO componente interativo:
   - Label associado a cada input
   - Foco visível (nunca outline:none sem alternativa)
   - Navegação por teclado funcional
   - aria-* quando o semântico HTML não cobre
   - Contraste verificado
8. Testes:
   - __tests__/*.test.tsx — queries acessíveis (getByRole, getByLabelText)
     — nunca getByTestId como primeira opção
   - Mock de I/O com handcraft, sem MSW no MVP a menos que a spec exija
9. Atualizar memória e spec (ver `reference/memory-and-output.md` §"Atualização de memória ao concluir")
```

A ordem é guia, não rígida — ajuste se a spec exigir.
