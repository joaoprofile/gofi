# Pattern — Safe Area

Bordas seguras (notch, status bar, home indicator, câmera). **Toda** tela respeita.

## Regras
- Use `react-native-safe-area-context`: `SafeAreaView` ou `useSafeAreaInsets()`.
- Conteúdo de tela: padding por insets (`top`, `bottom`). Listas: `contentContainerStyle`
  com `paddingBottom` ≥ inset inferior + tab bar.
- Header respeita `insets.top`; Tab Bar e CTAs fixos respeitam `insets.bottom`.
- Superfície de marca pode **sangrar** até a borda (full-bleed), mas o **conteúdo**
  (texto/ações) fica dentro da safe area.

## Componente Screen (base de toda tela)
```tsx
function Screen({ children }: { children: ReactNode }) {
  const insets = useSafeAreaInsets();
  const t = useTheme();
  return (
    <View style={{ flex:1, backgroundColor:t.surfacePage,
                   paddingTop:insets.top, paddingBottom:insets.bottom,
                   paddingHorizontal:t.space[4] }}>
      {children}
    </View>
  );
}
```

## StatusBar
- Ajuste `barStyle` ao fundo: conteúdo claro → `dark-content`; fundo escuro/marca
  escura → `light-content`. Sobre uma superfície de marca clara → `dark-content`.

## Do / Don't
- ✅ Testar em device com notch e com home indicator (gesture bar).
- ❌ Padding fixo "mágico" (ex.: 44) em vez de insets. ❌ CTA colado no home indicator.
