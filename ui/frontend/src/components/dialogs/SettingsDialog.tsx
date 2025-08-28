import { Dialog, DialogTitle, DialogContent, DialogActions, Button, Stack, TextField, Tooltip, Chip, Link } from '@mui/material';
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


