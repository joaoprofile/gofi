# Checkbox · Radio · Switch — mobile

Mesma semântica do [web](../../../web/gofi-ui/components/checkbox-radio-switch.md):
checkbox (0..N), radio (1 de N), switch (on/off **imediato**). No iOS, `Switch`
nativo é o padrão para liga/desliga.

## Props
```ts
interface ToggleProps { value: boolean; onValueChange: (v: boolean) => void;
  label: string; disabled?: boolean; }
```

## Acessibilidade
- Switch: `Switch` nativo (já tem `accessibilityRole="switch"` + estado), label como
  `Text` associado; a linha inteira é alvo (`Pressable` envolvendo, ≥ 44pt).
- Radio: `accessibilityRole="radio"` + `accessibilityState={{ checked }}`, agrupados.
- Checkbox: `accessibilityRole="checkbox"` + estado.

## Do / Don't
- ✅ Switch só p/ efeito imediato e reversível; anuncie o resultado.
- ✅ Toque na label inteira.
- ❌ Switch para algo que só vale após "salvar" (use checkbox).

## Exemplo
```tsx
<Pressable accessibilityRole="switch" onPress={() => setOn(!on)}>
  <Row><Text>Receber notificações</Text><Switch value={on} onValueChange={setOn} /></Row>
</Pressable>
```
