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
        // Catppuccin Mocha core palette
        primary: { main: '#cba6f7', contrastText: '#11111b' }, // mauve
        secondary: { main: '#89b4fa' }, // blue
        background: { default: '#1e1e2e', paper: '#181825' },
        text: { primary: '#cdd6f4', secondary: '#a6adc8' },
        divider: '#313244',
        error: { main: '#f38ba8' },
        warning: { main: '#fab387' },
        info: { main: '#89b4fa' },
        success: { main: '#a6e3a1' },
    },
    shape: { borderRadius: 12 },
    typography: {
        fontFamily:
            "Inter, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, 'Apple Color Emoji', 'Segoe UI Emoji'",
    },
    components: {
        MuiCssBaseline: {
            styleOverrides: {
                body: {
                    backgroundColor: '#1e1e2e',
                    color: '#cdd6f4',
                },
            },
        },
        MuiButton: {
            styleOverrides: {
                root: { textTransform: 'none' },
                containedPrimary: {
                    backgroundColor: '#cba6f7',
                    color: '#11111b',
                    '&:hover': { backgroundColor: '#dec7fa' },
                },
                outlined: {
                    borderColor: '#cba6f7',
                },
            },
        },
        MuiPaper: {
            styleOverrides: {
                root: {
                    backgroundImage: 'none',
                    boxShadow: '0 1px 2px rgba(0,0,0,0.4), 0 1px 3px rgba(0,0,0,0.6)',
                },
            },
        },
        MuiDivider: {
            styleOverrides: { root: { borderColor: '#313244' } },
        },
        MuiTooltip: {
            styleOverrides: {
                tooltip: { backgroundColor: '#313244', color: '#cdd6f4' },
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
    <ThemeProvider theme={theme}>
        <CssBaseline />
        <ErrorBoundary>
            <App />
        </ErrorBoundary>
    </ThemeProvider>
)
