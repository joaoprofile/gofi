# Feature List — mobile

Lista de **destaques** (ícone/check + texto curto) usada no card de marca do
onboarding (referência fiel do mockup). Não é navegação — é comunicação de valor.

## Anatomia
```
[✓] Texto da feature
[✓] Texto da feature
[✓] Texto da feature
```
Cada linha: ícone (check/marca) + `Text` body. Sobre superfície de marca → ícone e
texto em `textOnBrand`; sobre fundo branco → `colorAction`/`textColor`.

## Props
```ts
interface FeatureListProps { items: Array<{ icon?: ReactNode; label: string }>;
  onBrand?: boolean; }
```

## Acessibilidade
- `accessibilityRole="list"` (container) / item com label = texto da feature.
- Ícone decorativo oculto do leitor; o texto carrega o significado.
- Contraste: sobre a superfície de marca clara use `textOnBrand` (navy), nunca branco.

## Do / Don't
- ✅ Frases curtas, escaneáveis (3–5 itens). ✅ Ícone consistente.
- ❌ Parágrafos longos. ❌ Usar como menu de navegação.

## Exemplo
```tsx
<FeatureList onBrand items={[
  { label: 'Recurso principal um' },
  { label: 'Recurso principal dois' },
  { label: 'Recurso principal três' },
]} />
```
