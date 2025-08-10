import { Dialog, DialogTitle, DialogContent, DialogActions, Button, Stack, TextField, Typography } from '@mui/material';

type Props = {
    open: boolean;
    workspacePath: string;
    setWorkspacePath: (v: string) => void;
    onBrowse: () => void;
    onUse: () => void;
    onClose: () => void;
};

export default function WorkspaceDialog({ open, workspacePath, setWorkspacePath, onBrowse, onUse, onClose }: Props) {
    return (
        <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
            <DialogTitle>Select Workspace</DialogTitle>
            <DialogContent dividers>
                <Stack spacing={2} sx={{ mt: 1 }}>
                    <Stack direction="row" spacing={1}>
                        <TextField label="Workspace Path" value={workspacePath} onChange={(e) => setWorkspacePath(e.target.value)} placeholder="/path/to/project" fullWidth />
                        <Button variant="outlined" onClick={onBrowse}>Browse…</Button>
                    </Stack>
                    <Typography variant="body2" color="text.secondary">
                        Enter a project directory. Project rules will be stored in <code>.loom/rules.json</code> under this path.
                    </Typography>
                </Stack>
            </DialogContent>
            <DialogActions>
                <Button onClick={onClose} color="inherit">Cancel</Button>
                <Button variant="contained" onClick={onUse} disabled={!workspacePath.trim()}>Use</Button>
            </DialogActions>
        </Dialog>
    );
}


