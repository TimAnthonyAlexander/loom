import { createTheme, Theme } from '@mui/material/styles';

const catppuccinTheme = createTheme({
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
        MuiTypography: {
            styleOverrides: {
                root: {
                    color: '#cdd6f4',
                },
            },
        },
    },
});

const tealTheme = createTheme({
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
                    backgroundColor: '#26a69a',
                    color: '#000000',
                    '&:hover': { backgroundColor: '#4db6ac' },
                },
                outlined: {
                    borderColor: '#26a69a',
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
            styleOverrides: { root: { borderColor: '#37474f' } },
        },
        MuiTooltip: {
            styleOverrides: {
                tooltip: { backgroundColor: '#37474f', color: '#ffffff' },
            },
        },
        MuiTypography: {
            styleOverrides: {
                root: {
                    color: '#ffffff',
                },
            },
        },
    },
});

export const getTheme = (themeName: string): Theme => {
    switch (themeName) {
        case 'catppuccin':
            return catppuccinTheme;
        case 'teal':
            return tealTheme;
        default:
            return catppuccinTheme;
    }
};
