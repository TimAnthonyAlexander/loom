import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import ErrorBoundary from './ErrorBoundary'
import { LogError } from '../wailsjs/runtime/runtime'
import './App.css'
import { ThemeProvider, createTheme, CssBaseline } from '@mui/material'
import './monaco-workers'

const theme = createTheme({
    palette: {
        mode: 'dark',
        primary: { main: '#0A84FF' },
        background: { default: '#0B0D12', paper: '#121417' },
    },
    shape: { borderRadius: 12 },
    typography: {
        fontFamily:
            "Inter, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, 'Apple Color Emoji', 'Segoe UI Emoji'",
    },
    components: {
        MuiButton: {
            styleOverrides: { root: { textTransform: 'none' } },
        },
        MuiPaper: {
            styleOverrides: {
                root: {
                    boxShadow:
                        '0 1px 2px rgba(0,0,0,0.4), 0 1px 3px rgba(0,0,0,0.6)',
                },
            },
        },
    },
})

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
        <ThemeProvider theme={theme}>
            <CssBaseline />
            <ErrorBoundary>
                <App />
            </ErrorBoundary>
        </ThemeProvider>
    </React.StrictMode>,
)
