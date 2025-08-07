import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import ErrorBoundary from './ErrorBoundary'
import { LogError } from '../wailsjs/runtime/runtime'
import './App.css'

// Global error handlers
window.addEventListener('error', (event) => {
  const errorMessage = `Global Error: ${event.message}\nFile: ${event.filename}\nLine: ${event.lineno}\nColumn: ${event.colno}\nStack: ${event.error?.stack}`;
  
  console.error('Global error caught:', event);
  
  try {
    LogError(errorMessage);
  } catch (e) {
    console.error('Failed to log to Wails runtime:', e);
  }
});

window.addEventListener('unhandledrejection', (event) => {
  const errorMessage = `Unhandled Promise Rejection: ${event.reason}\nStack: ${event.reason?.stack || 'No stack available'}`;
  
  console.error('Unhandled promise rejection:', event.reason);
  
  try {
    LogError(errorMessage);
  } catch (e) {
    console.error('Failed to log to Wails runtime:', e);
  }
});

ReactDOM.createRoot(document.getElementById('root') as HTMLElement).render(
  <React.StrictMode>
    <ErrorBoundary>
      <App />
    </ErrorBoundary>
  </React.StrictMode>,
)