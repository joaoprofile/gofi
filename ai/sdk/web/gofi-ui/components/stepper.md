# Stepper

Um fluxo sequencial multi-etapa (onboarding, checkout, formulário longo). Mostra onde o
usuário está e quanto falta (princípio 12, time-to-value).

## Anatomia
`[ ① feito ]—[ ② atual ]—[ ③ pendente ]` (horizontal) ou vertical com uma
descrição. Etapa feita: `--success`/check; atual: `--action`; pendente:
`--sf-border`.

## Variantes
`horizontal` · `vertical` · `numbered` · `dot` (compacto). Navegável (clique nas
etapas concluídas) ou estritamente linear.

## Estados (por etapa)
`completed · current · upcoming · error (etapa com problema) · disabled`.

## Props
```ts
interface Step { id: string; label: string; optional?: boolean; status?: 'error'; }
interface StepperProps { steps: Step[]; current: number; orientation?: 'horizontal'|'vertical';
  onStepClick?: (i: number) => void; }
```

## Acessibilidade
- `<ol>` com `aria-current="step"` na atual. Estado via ícone+texto, não só
  cor.
- Em formulários, cada etapa é uma seção com um título; erros movem o foco para o campo.
- Navegação por teclado entre as etapas concluídas.

## Do / Don't
- ✅ Permitir voltar sem perder dados já preenchidos.
- ✅ Mostrar o progresso ("Etapa 2 de 4").
- ❌ Avançar uma etapa inválida silenciosamente. ❌ Stepper para navegação não-linear (use Tabs).

## Exemplo
```tsx
<Stepper current={1} steps={[{id:'a',label:'Detalhes'},{id:'b',label:'Endereço'},{id:'c',label:'Revisão'}]} />
```
