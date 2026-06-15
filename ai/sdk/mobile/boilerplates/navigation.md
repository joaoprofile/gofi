# Boilerplate — navegação + providers (mobile)

React Navigation para roteamento; `ThemeProvider` do DS para tema/marca; `TabBar`/
`Header` do DS são a camada visual.

## `app/App.tsx`
```tsx
import { ThemeProvider } from 'gofi-ui-native';
import { NavigationContainer } from '@react-navigation/native';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { RootNavigator } from '../navigation/RootNavigator';

const queryClient = new QueryClient();

export default function App() {
  return (
    <ThemeProvider brand={brand}>     {/* cores de ui.brand do .gofi.yaml; omitido → padrão neutro */}
      <QueryClientProvider client={queryClient}>
        <NavigationContainer>
          <RootNavigator />
        </NavigationContainer>
      </QueryClientProvider>
    </ThemeProvider>
  );
}
```

## `navigation/TabNavigator.tsx`
```tsx
import { createBottomTabNavigator } from '@react-navigation/bottom-tabs';
import { <Feature>Screen } from '../screens/<Feature>Screen';

const Tab = createBottomTabNavigator();

export function TabNavigator() {
  return (
    <Tab.Navigator screenOptions={{ headerShown: false }}>
      <Tab.Screen name="{contexto}" component={<Feature>Screen} options={{ title: 'Início' }} />
      {/* 3–5 destinos no máximo */}
    </Tab.Navigator>
  );
}
```

## `navigation/RootNavigator.tsx`
```tsx
import { createNativeStackNavigator } from '@react-navigation/native-stack';
import { TabNavigator } from './TabNavigator';

const Stack = createNativeStackNavigator();

export function RootNavigator() {
  return (
    <Stack.Navigator screenOptions={{ headerShown: false }}>
      <Stack.Screen name="Tabs" component={TabNavigator} />
      {/* telas de detalhe empilhadas aqui */}
    </Stack.Navigator>
  );
}
```

> `ThemeProvider` + `NavigationContainer` envolvem a app **uma vez** em `App.tsx`.
> Tab bar com 3–5 destinos (`patterns/navigation.md`). Telas dentro do `Screen`
> (safe-area) do DS.
</content>
