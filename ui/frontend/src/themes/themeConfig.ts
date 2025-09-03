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

const lightTheme = createTheme({
    palette: {
        mode: 'light',
        // Clean light theme with blue accents
        primary: { main: '#1976d2', contrastText: '#ffffff' }, // blue
        secondary: { main: '#f57c00', contrastText: '#ffffff' }, // orange
        background: { default: '#ffffff', paper: '#f5f5f5' },
        text: { primary: '#000000', secondary: '#666666' },
        divider: '#e0e0e0',
        error: { main: '#d32f2f' },
        warning: { main: '#f57c00' },
        info: { main: '#1976d2' },
        success: { main: '#388e3c' },
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
                    backgroundColor: '#ffffff',
                    color: '#000000',
                },
            },
        },
        MuiButton: {
            styleOverrides: {
                root: { textTransform: 'none' },
                containedPrimary: {
                    backgroundColor: '#1976d2',
                    color: '#ffffff',
                    '&:hover': { backgroundColor: '#1565c0' },
                },
                outlined: {
                    borderColor: '#1976d2',
                },
            },
        },
        MuiPaper: {
            styleOverrides: {
                root: {
                    backgroundImage: 'none',
                    boxShadow: '0 1px 3px rgba(0,0,0,0.12), 0 1px 2px rgba(0,0,0,0.24)',
                },
            },
        },
        MuiDivider: {
            styleOverrides: { root: { borderColor: '#e0e0e0' } },
        },
        MuiTooltip: {
            styleOverrides: {
                tooltip: { backgroundColor: '#616161', color: '#ffffff' },
            },
        },
        MuiTypography: {
            styleOverrides: {
                root: {
                    color: '#000000',
                },
            },
        },
    },
});

const purpleTheme = createTheme({
    palette: {
        mode: 'dark',
        // Dark purple theme
        primary: { main: '#9c27b0', contrastText: '#ffffff' }, // purple
        secondary: { main: '#e91e63', contrastText: '#ffffff' }, // pink
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
                    backgroundColor: '#9c27b0',
                    color: '#ffffff',
                    '&:hover': { backgroundColor: '#7b1fa2' },
                },
                outlined: {
                    borderColor: '#9c27b0',
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

const forestTheme = createTheme({
    palette: {
        mode: 'dark',
        // Forest green theme
        primary: { main: '#388e3c', contrastText: '#ffffff' }, // forest green
        secondary: { main: '#ff9800', contrastText: '#000000' }, // amber
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
                    backgroundColor: '#388e3c',
                    color: '#ffffff',
                    '&:hover': { backgroundColor: '#2e7d32' },
                },
                outlined: {
                    borderColor: '#388e3c',
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

// Theme registry for easy extensibility
// To add a new theme:
// 1. Create a Material-UI theme object (copy existing theme as starting point)
// 2. Copy an existing *_converted.json file and replace colors with search/replace
// 3. Add both to this registry and the EditorPanel themeMap
export const AVAILABLE_THEMES = {
    catppuccin: { name: 'Catppuccin', theme: catppuccinTheme, mode: 'dark' },
    teal: { name: 'Teal', theme: tealTheme, mode: 'dark' },
    light: { name: 'Light', theme: lightTheme, mode: 'light' },
    purple: { name: 'Purple', theme: purpleTheme, mode: 'dark' },
    forest: { name: 'Forest', theme: forestTheme, mode: 'dark' },
} as const;

export type ThemeName = keyof typeof AVAILABLE_THEMES;

export const getTheme = (themeName: string): Theme => {
    const themeConfig = AVAILABLE_THEMES[themeName as ThemeName];
    return themeConfig?.theme || AVAILABLE_THEMES.catppuccin.theme;
};
