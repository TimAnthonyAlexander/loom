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
        // Custom Dark Teal & Amber palette
        primary: { main: '#26a69a', contrastText: '#000000' }, // teal
        secondary: { main: '#ffca28', contrastText: '#000000' }, // amber
        background: { default: '#121212', paper: '#1e1e1e' },
        text: { primary: '#ffffff', secondary: '#b0bec5' },
        divider: '#37474f',
        error: { main: '#ef5350' },
        warning: { main: '#ffa726' },
        info: { main: '#29b6f6' },
        success: { main: '#66bb6a' },
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

                    backgroundColor: '#121212',
                    color: '#ffffff',
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
