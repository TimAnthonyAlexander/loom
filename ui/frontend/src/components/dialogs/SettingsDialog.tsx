import React from 'react';
import {
    Dialog,
    DialogTitle,
    DialogContent,
    DialogActions,
    Button,
    Stack,
    TextField,
    Tooltip,
    Link,
    FormControl,
    InputLabel,
    Select,
    MenuItem,
    FormControlLabel,
    Switch,
    Typography,
    Box,
    IconButton,
    InputAdornment
} from '@mui/material';
import KeyIcon from '@mui/icons-material/VpnKey';
import Visibility from '@mui/icons-material/Visibility';
import VisibilityOff from '@mui/icons-material/VisibilityOff';
import LaunchIcon from '@mui/icons-material/Launch';
import { AVAILABLE_THEMES } from '../../themes/themeConfig';
import { OpenProjectDataDir } from '../../../wailsjs/go/bridge/App';

type Props = {
    open: boolean;
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
    onClose: () => void;
};

export default function SettingsDialog({
    open,
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
    onSave,
    onClose
}: Props) {
    const [showOpenAI, setShowOpenAI] = React.useState(false);
    const [showAnthropic, setShowAnthropic] = React.useState(false);
    const [showOpenRouter, setShowOpenRouter] = React.useState(false);

    const SectionTitle = ({ children }: { children: React.ReactNode }) => (
        <Typography variant="overline" sx={{ letterSpacing: 0.6, opacity: 0.8 }}>
            {children}
        </Typography>
    );

    const keyAdornment = (
        <InputAdornment position="start">
            <KeyIcon fontSize="small" />
        </InputAdornment>
    );

    return (
        <Dialog
            open={open}
            onClose={onClose}
            maxWidth="sm"
            fullWidth
            PaperProps={{
                sx: {
                    borderRadius: 2,
                    boxShadow: '0 10px 30px rgba(0,0,0,0.35)'
                }
            }}
        >
            <DialogTitle sx={{ fontWeight: 700, pb: 0.5 }}>Settings</DialogTitle>

            <DialogContent sx={{ pt: 1.5 }}>
                <Stack spacing={3}>
                    <Box>
                        <SectionTitle>API Keys</SectionTitle>
                        <Stack spacing={1.25} sx={{ mt: 1 }}>
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
                    </Box>

                    <Box>
                        <SectionTitle>Local Models</SectionTitle>
                        <Stack spacing={1.25} sx={{ mt: 1 }}>
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
                                sx={{ alignSelf: 'flex-start', display: 'inline-flex', alignItems: 'center', gap: 0.5 }}
                            >
                                Open project data folder <LaunchIcon fontSize="small" />
                            </Link>
                        </Stack>
                    </Box>

                    <Box>
                        <SectionTitle>Appearance</SectionTitle>
                        <FormControl fullWidth sx={{ mt: 1 }}>
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
                    </Box>

                    <Box>
                        <SectionTitle>Automation</SectionTitle>
                        <Stack spacing={1} sx={{ mt: 1 }}>
                            <Tooltip title="If enabled, shell commands proposed by the model are executed without manual approval.">
                                <FormControlLabel
                                    control={
                                        <Switch
                                            checked={autoApproveShell}
                                            onChange={() => setAutoApproveShell(!autoApproveShell)}
                                            size="small"
                                        />
                                    }
                                    label="Auto-Approve Shell"
                                />
                            </Tooltip>
                            <Tooltip title="If enabled, file edits are applied without manual approval.">
                                <FormControlLabel
                                    control={
                                        <Switch
                                            checked={autoApproveEdits}
                                            onChange={() => setAutoApproveEdits(!autoApproveEdits)}
                                            size="small"
                                        />
                                    }
                                    label="Auto-Approve Edits"
                                />
                            </Tooltip>
                        </Stack>
                    </Box>
                </Stack>
            </DialogContent>

            <DialogActions sx={{ px: 3, pb: 2 }}>
                <Button onClick={onClose} color="inherit">Close</Button>
                <Button variant="contained" onClick={onSave}>Save</Button>
            </DialogActions>
        </Dialog>
    );
}
