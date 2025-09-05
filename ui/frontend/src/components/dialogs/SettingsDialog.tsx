import { Dialog, DialogTitle, DialogContent, DialogActions, Button, Stack, TextField, Tooltip, Chip, Link, FormControl, InputLabel, Select, MenuItem } from '@mui/material';
import { OpenProjectDataDir } from '../../../wailsjs/go/bridge/App';
import { AVAILABLE_THEMES } from '../../themes/themeConfig';

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
    currentPersonality: string;
    setCurrentPersonality: (v: string) => void;
    personalities: Record<string, { name: string; description: string; prompt: string }>;
    onSave: () => void;
    onClose: () => void;
};

export default function SettingsDialog(props: Props) {
    const {
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
        currentPersonality,
        setCurrentPersonality,
        personalities,
        onSave,
        onClose,
    } = props;

    return (
        <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
            <DialogTitle>Settings</DialogTitle>
            <DialogContent dividers>
                <Stack spacing={2} sx={{ mt: 1 }}>
                    <TextField label="OpenAI API Key" type="password" autoComplete="off" value={openaiKey} onChange={(e) => setOpenaiKey(e.target.value)} placeholder="sk-..." fullWidth />
                    <TextField label="Anthropic API Key" type="password" autoComplete="off" value={anthropicKey} onChange={(e) => setAnthropicKey(e.target.value)} placeholder="sk-ant-..." fullWidth />
                    <TextField label="OpenRouter API Key" type="password" autoComplete="off" value={openrouterKey} onChange={(e) => setOpenrouterKey(e.target.value)} placeholder="sk-or-..." fullWidth />
                    <TextField label="Ollama Endpoint" value={ollamaEndpoint} onChange={(e) => setOllamaEndpoint(e.target.value)} placeholder="http://localhost:11434" fullWidth />
                    <FormControl fullWidth>
                        <InputLabel>Theme</InputLabel>
                        <Select value={currentTheme} onChange={(e) => setCurrentTheme(e.target.value)} label="Theme">
                            {Object.entries(AVAILABLE_THEMES).map(([key, config]) => (
                                <MenuItem key={key} value={key}>{config.name}</MenuItem>
                            ))}
                        </Select>
                    </FormControl>
                    <FormControl fullWidth>
                        <InputLabel>Personality</InputLabel>
                        <Select value={currentPersonality} onChange={(e) => setCurrentPersonality(e.target.value)} label="Personality">
                            {[
                                'coder',      // The Coder (default, most common)
                                'architect',  // The Architect (planning-heavy)
                                'debugger',   // The Debugger (problem-solving)
                                'reviewer',   // The Reviewer (review and quality)
                                'founder',    // The Founder (business-driven)
                                'scientist',  // Mad Scientist (playful but still technical)
                                'comedian',   // Stand-up Comedian
                                'pirate',     // Pirate Captain
                                'bavarian',   // The Bavarian Boy
                                'waifu',      // Anime Waifu
                            ]
                                .filter(key => personalities[key]) // Only show personalities that exist
                                .map((key) => {
                                    const config = personalities[key];
                                    return (
                                        <MenuItem key={key} value={key}>
                                            <Tooltip title={config.description} placement="right">
                                                <span>{config.name}</span>
                                            </Tooltip>
                                        </MenuItem>
                                    );
                                })}
                        </Select>
                    </FormControl>
                    <Stack direction="row" spacing={2} alignItems="center">
                        <Link component="button" underline="hover" onClick={() => OpenProjectDataDir()}>
                            Open project data folder
                        </Link>
                    </Stack>
                    <Stack direction="row" spacing={2} alignItems="center">
                        <Tooltip title="If enabled, shell commands proposed by the model are executed without manual approval.">
                            <Chip label="Auto-Approve Shell" />
                        </Tooltip>
                        <Button variant={autoApproveShell ? 'contained' : 'outlined'} onClick={() => setAutoApproveShell(!autoApproveShell)}>
                            {autoApproveShell ? 'On' : 'Off'}
                        </Button>
                    </Stack>
                    <Stack direction="row" spacing={2} alignItems="center">
                        <Tooltip title="If enabled, file edits are applied without manual approval.">
                            <Chip label="Auto-Approve Edits" />
                        </Tooltip>
                        <Button variant={autoApproveEdits ? 'contained' : 'outlined'} onClick={() => setAutoApproveEdits(!autoApproveEdits)}>
                            {autoApproveEdits ? 'On' : 'Off'}
                        </Button>
                    </Stack>
                </Stack>
            </DialogContent>
            <DialogActions>
                <Button onClick={onClose} color="inherit">Close</Button>
                <Button variant="contained" onClick={onSave}>Save</Button>
            </DialogActions>
        </Dialog>
    );
}


