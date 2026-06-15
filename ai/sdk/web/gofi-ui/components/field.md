# Field

Wrapper de **todo** controle de formulário: associa label, hint e mensagem de erro
ao input. Base de Input, Select, etc. Garante a regra "todo input tem label + erro +
hint".

## Anatomia
```
Label (obrigatório)        [• opcional/obrigatório]
[ controle ................................. ]
Hint (apoio)  |  Erro (substitui o hint quando inválido)
```

## Estados
`default · focus · invalid (borda --danger + mensagem) · disabled · readonly`.

## Props
```ts
interface FieldProps {
  label: string;                 // sempre visível; um placeholder NÃO substitui
  htmlFor: string;               // id do controle (associação a11y)
  hint?: string;                 // ajuda neutra
  error?: string;                // mensagem de erro (útil e específica)
  required?: boolean;            // marca visual + aria-required no controle
  children: ReactNode;           // o controle (Input/Select/…)
}
```

## Acessibilidade
- `<label htmlFor={id}>` sempre presente e visível.
- Erro: o controle recebe `aria-invalid="true"` + `aria-describedby` apontando para o
  id da mensagem. O hint também via `aria-describedby`.
- `required` → `aria-required="true"` (não dependa só do asterisco).

## Do / Don't
- ✅ Mensagem de erro **específica e acionável**: "Informe um e-mail válido", não "campo inválido".
- ✅ O erro aparece após a interação (blur/submit), não enquanto o usuário digita o
  primeiro caractere.
- ❌ Placeholder como label. ❌ Erro só por borda vermelha (sem texto).

## Exemplo
```tsx
<Field label="E-mail" htmlFor="email" error={errors.email} required
       hint="Usamos para enviar seu acesso.">
  <Input id="email" type="email" aria-invalid={!!errors.email} />
</Field>
```
