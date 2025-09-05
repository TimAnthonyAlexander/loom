import React from 'react';
import {
    Box,
    Stack,
    TextField,
    Link,
    FormControl,
    InputLabel,
    Select,
    MenuItem,
    FormControlLabel,
    Switch,
    Typography,
    IconButton,
    InputAdornment,
    Paper
} from '@mui/material';
import KeyIcon from '@mui/icons-material/VpnKey';
import Visibility from '@mui/icons-material/Visibility';
import VisibilityOff from '@mui/icons-material/VisibilityOff';
import LaunchIcon from '@mui/icons-material/Launch';
import { AVAILABLE_THEMES } from '../../themes/themeConfig';
import { OpenProjectDataDir } from '../../../wailsjs/go/bridge/App';

type Props = {
    openaiKey: string;
    setOpenaiKey: (v: string) => void;
    anthropicKey: string;
    setAnthropicKey: (v: string) => void;
    openrouterKey: string;
    setOpenrouterKey: (v: string) => void;
    ollamaEndpoint: string;
    setOllamaEndpoint: (v: string) => void;
    autoApproveShell: boolean;
    setAutoApproveShell: (v: boolean) => void;
    autoApproveEdits: boolean;
    setAutoApproveEdits: (v: boolean) => void;
    currentTheme: string;
    setCurrentTheme: (v: string) => void;
    onSave: () => void;
};

export default function SettingsTab({
    openaiKey,
    setOpenaiKey,
    anthropicKey,
    setAnthropicKey,
    openrouterKey,
    setOpenrouterKey,
    ollamaEndpoint,
    setOllamaEndpoint,
    autoApproveShell,
    setAutoApproveShell,
    autoApproveEdits,
    setAutoApproveEdits,
    currentTheme,
    setCurrentTheme,
    onSave
}: Props) {
    const [showOpenAI, setShowOpenAI] = React.useState(false);
    const [showAnthropic, setShowAnthropic] = React.useState(false);
    const [showOpenRouter, setShowOpenRouter] = React.useState(false);

    // Auto-save changes after a debounce period
    const saveTimeoutRef = React.useRef<NodeJS.Timeout | null>(null);
    
    const debouncedSave = React.useCallback(() => {
        if (saveTimeoutRef.current) {
            clearTimeout(saveTimeoutRef.current);
        }
        saveTimeoutRef.current = setTimeout(() => {
            onSave();
        }, 1000);
    }, [onSave]);

    // Auto-save when any setting changes
    React.useEffect(() => {
        debouncedSave();
    }, [openaiKey, anthropicKey, openrouterKey, ollamaEndpoint, autoApproveShell, autoApproveEdits, currentTheme, debouncedSave]);

    const SectionTitle = ({ children }: { children: React.ReactNode }) => (
        <Typography variant="h6" sx={{ 
            fontWeight: 600, 
            mb: 2, 
            color: 'text.primary',
            borderBottom: 1,
            borderColor: 'divider',
            pb: 1
        }}>
            {children}
        </Typography>
    );

    const keyAdornment = (
        <InputAdornment position="start">
            <KeyIcon fontSize="small" />
        </InputAdornment>
    );

    return (
        <Box sx={{ 
            height: '100%', 
            overflow: 'auto',
            bgcolor: 'background.default'
        }}>
            <Box sx={{ p: 3, maxWidth: 800 }}>
                <Typography 
                    variant="h4" 
                    sx={{ 
                        fontWeight: 700, 
                        mb: 4,
                        background: 'linear-gradient(45deg, currentColor, primary.main)',
                        backgroundClip: 'text',
                        WebkitBackgroundClip: 'text',
                        color: 'transparent'
                    }}
                >
                    Settings
                </Typography>

                <Stack spacing={4}>
                    <Paper elevation={0} sx={{ p: 3, border: 1, borderColor: 'divider', borderRadius: 2 }}>
                        <SectionTitle>API Keys</SectionTitle>
                        <Stack spacing={2.5}>
                            <TextField
                                label="OpenAI API Key"
                                type={showOpenAI ? 'text' : 'password'}
                                autoComplete="off"
                                value={openaiKey}
                                onChange={(e) => setOpenaiKey(e.target.value)}
                                placeholder="sk-..."
                                fullWidth
                                InputProps={{
                                    startAdornment: keyAdornment,
                                    endAdornment: (
                                        <InputAdornment position="end">
                                            <IconButton onClick={() => setShowOpenAI((v) => !v)} edge="end">
                                                {showOpenAI ? <VisibilityOff /> : <Visibility />}
                                            </IconButton>
                                        </InputAdornment>
                                    )
                                }}
                            />
                            <TextField
                                label="Anthropic API Key"
                                type={showAnthropic ? 'text' : 'password'}
                                autoComplete="off"
                                value={anthropicKey}
                                onChange={(e) => setAnthropicKey(e.target.value)}
                                placeholder="sk-ant-..."
                                fullWidth
                                InputProps={{
                                    startAdornment: keyAdornment,
                                    endAdornment: (
                                        <InputAdornment position="end">
                                            <IconButton onClick={() => setShowAnthropic((v) => !v)} edge="end">
                                                {showAnthropic ? <VisibilityOff /> : <Visibility />}
                                            </IconButton>
                                        </InputAdornment>
                                    )
                                }}
                            />
                            <TextField
                                label="OpenRouter API Key"
                                type={showOpenRouter ? 'text' : 'password'}
                                autoComplete="off"
                                value={openrouterKey}
                                onChange={(e) => setOpenrouterKey(e.target.value)}
                                placeholder="sk-or-..."
                                fullWidth
                                InputProps={{
                                    startAdornment: keyAdornment,
                                    endAdornment: (
                                        <InputAdornment position="end">
                                            <IconButton onClick={() => setShowOpenRouter((v) => !v)} edge="end">
                                                {showOpenRouter ? <VisibilityOff /> : <Visibility />}
                                            </IconButton>
                                        </InputAdornment>
                                    )
                                }}
                            />
                        </Stack>
                    </Paper>

                    <Paper elevation={0} sx={{ p: 3, border: 1, borderColor: 'divider', borderRadius: 2 }}>
                        <SectionTitle>Local Models</SectionTitle>
                        <Stack spacing={2.5}>
                            <TextField
                                label="Ollama Endpoint"
                                value={ollamaEndpoint}
                                onChange={(e) => setOllamaEndpoint(e.target.value)}
                                placeholder="http://localhost:11434"
                                fullWidth
                                InputProps={{ sx: { fontFamily: 'ui-monospace, Menlo, monospace' } }}
                            />
                            <Link
                                component="button"
                                underline="hover"
                                onClick={() => OpenProjectDataDir()}
                                sx={{ 
                                    alignSelf: 'flex-start', 
                                    display: 'inline-flex', 
                                    alignItems: 'center', 
                                    gap: 0.5,
                                    textAlign: 'left'
                                }}
                            >
                                Open project data folder <LaunchIcon fontSize="small" />
                            </Link>
                        </Stack>
                    </Paper>

                    <Paper elevation={0} sx={{ p: 3, border: 1, borderColor: 'divider', borderRadius: 2 }}>
                        <SectionTitle>Appearance</SectionTitle>
                        <FormControl fullWidth>
                            <InputLabel shrink>Theme</InputLabel>
                            <Select
                                value={currentTheme}
                                label="Theme"
                                onChange={(e) => setCurrentTheme(e.target.value)}
                                MenuProps={{ PaperProps: { sx: { borderRadius: 2 } } }}
                            >
                                {Object.entries(AVAILABLE_THEMES).map(([key, config]) => (
                                    <MenuItem key={key} value={key} sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                                        {config.name}
                                    </MenuItem>
                                ))}
                            </Select>
                        </FormControl>
                    </Paper>

                    <Paper elevation={0} sx={{ p: 3, border: 1, borderColor: 'divider', borderRadius: 2 }}>
                        <SectionTitle>Automation</SectionTitle>
                        <Stack spacing={2}>
                            <Box sx={{ 
                                p: 2, 
                                bgcolor: 'action.hover', 
                                borderRadius: 1,
                                border: 1,
                                borderColor: 'divider'
                            }}>
                                <FormControlLabel
                                    control={
                                        <Switch
                                            checked={autoApproveShell}
                                            onChange={() => setAutoApproveShell(!autoApproveShell)}
                                            size="medium"
                                        />
                                    }
                                    label={
                                        <Box>
                                            <Typography variant="body1" fontWeight={600}>
                                                Auto-Approve Shell Commands
                                            </Typography>
                                            <Typography variant="body2" color="text.secondary">
                                                Shell commands proposed by the model are executed without manual approval
                                            </Typography>
                                        </Box>
                                    }
                                    sx={{ alignItems: 'flex-start', m: 0 }}
                                />
                            </Box>
                            <Box sx={{ 
                                p: 2, 
                                bgcolor: 'action.hover', 
                                borderRadius: 1,
                                border: 1,
                                borderColor: 'divider'
                            }}>
                                <FormControlLabel
                                    control={
                                        <Switch
                                            checked={autoApproveEdits}
                                            onChange={() => setAutoApproveEdits(!autoApproveEdits)}
                                            size="medium"
                                        />
                                    }
                                    label={
                                        <Box>
                                            <Typography variant="body1" fontWeight={600}>
                                                Auto-Approve File Edits
                                            </Typography>
                                            <Typography variant="body2" color="text.secondary">
                                                File edits are applied without manual approval
                                            </Typography>
                                        </Box>
                                    }
                                    sx={{ alignItems: 'flex-start', m: 0 }}
                                />
                            </Box>
                        </Stack>
                    </Paper>
                </Stack>

                <Box sx={{ mt: 4, p: 2, bgcolor: 'info.main', color: 'info.contrastText', borderRadius: 2 }}>
                    <Typography variant="body2">
                        💡 Settings are automatically saved as you make changes
                    </Typography>
                </Box>
            </Box>
        </Box>
    );
}
