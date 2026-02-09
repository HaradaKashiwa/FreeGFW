import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import App from './App.jsx'
import './styles/tailwind.css'
import { setConfig } from './utils/fetcher.js'

setConfig({
  api: import.meta.env.DEV ? 'http://localhost:4444' : window.location.origin
})

createRoot(document.getElementById('root')).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
