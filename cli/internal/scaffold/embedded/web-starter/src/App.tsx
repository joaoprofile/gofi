import { useState } from 'react'
import { Button } from 'gofi-ui'
import './App.css'

export default function App() {
  const [message, setMessage] = useState('')

  return (
    <main className="hello">
      <section className="hello__card">
        <h1 className="hello__title">Hello, gofi-ui 👋</h1>
        <p className="hello__message">{message || 'Clique no botão para começar.'}</p>
        <Button
          variant="primary"
          size="lg"
          onClick={() => setMessage('Hello, world! 🎉')}
        >
          Dizer olá
        </Button>
      </section>
    </main>
  )
}
