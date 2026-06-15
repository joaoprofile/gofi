import { useState } from 'react'
import { StyleSheet, Text, View } from 'react-native'
import { StatusBar } from 'expo-status-bar'
import { ThemeProvider, Button } from 'gofi-ui-native'

function HelloScreen() {
  const [message, setMessage] = useState('')

  return (
    <View style={styles.screen}>
      <Text style={styles.title}>Hello, gofi-ui-native 👋</Text>
      <Text style={styles.message}>{message || 'Toque no botão para começar.'}</Text>
      <Button
        variant="primary"
        full
        onPress={() => setMessage('Hello, world! 🎉')}
      >
        Dizer olá
      </Button>
      <StatusBar style="auto" />
    </View>
  )
}

export default function App() {
  return (
    <ThemeProvider>
      <HelloScreen />
    </ThemeProvider>
  )
}

const styles = StyleSheet.create({
  screen: {
    flex: 1,
    alignItems: 'center',
    justifyContent: 'center',
    gap: 16,
    paddingHorizontal: 24,
    backgroundColor: '#ffffff',
  },
  title: {
    fontSize: 24,
    fontWeight: '700',
    color: '#0b1f3a',
    textAlign: 'center',
  },
  message: {
    fontSize: 16,
    color: '#475569',
    textAlign: 'center',
    minHeight: 22,
  },
})
