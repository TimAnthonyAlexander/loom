import React, { ReactNode } from 'react';
import { ThemeProvider } from '@mui/material/styles';
import { getTheme } from '../themes/themeConfig';

interface DynamicThemeProviderProps {
    currentTheme: string;
    children: ReactNode;
}

export const DynamicThemeProvider: React.FC<DynamicThemeProviderProps> = ({ currentTheme, children }) => {
    const theme = getTheme(currentTheme);

    return (
        <ThemeProvider theme={theme}>
            {children}
        </ThemeProvider>
    );
};
