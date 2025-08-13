import { Dialog, DialogTitle, DialogContent, DialogActions, Button, Stack, Paper, Typography, IconButton } from '@mui/material';
import DeleteIcon from '@mui/icons-material/DeleteOutline';
import { useEffect, useState } from 'react';
import * as AppBridge from '../../../wailsjs/go/bridge/App';

type Props = {
    open: boolean;
    onClose: () => void;
};

type Memory = { id: string; text: string };

export default function MemoriesDialog(props: Props) {
    const { open, onClose } = props;
    const [memories, setMemories] = useState<Memory[]>([]);

    const refresh = () => {
        (AppBridge as any).GetMemories?.().then((list: any) => {
            const arr = Array.isArray(list) ? list : [];
            setMemories(arr.map((m: any) => ({ id: String(m?.id || ''), text: String(m?.text || '') })));
        }).catch(() => { setMemories([]); });
    };

    useEffect(() => {
        if (open) refresh();
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [open]);

    const onDelete = (id: string) => {
        (AppBridge as any).DeleteMemory?.(id).then((ok: boolean) => {
            if (ok) setMemories(prev => prev.filter(m => m.id !== id));
        }).catch(() => {});
    };

    return (
        <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
            <DialogTitle>Memories</DialogTitle>
            <DialogContent dividers>
                <Stack spacing={1} sx={{ mt: 1 }}>
                    {memories.length === 0 && (
                        <Typography variant="body2" color="text.secondary">No memories saved yet.</Typography>
                    )}
                    {memories.map((m) => (
                        <Paper key={m.id} variant="outlined" sx={{ p: 1 }}>
                            <Stack direction="row" spacing={1} alignItems="center">
                                <Stack sx={{ flex: 1 }}>
                                    <Typography variant="caption" color="text.secondary">{m.id}</Typography>
                                    <Typography variant="body2">{m.text}</Typography>
                                </Stack>
                                <IconButton size="small" color="error" onClick={() => onDelete(m.id)}>
                                    <DeleteIcon fontSize="small" />
                                </IconButton>
                            </Stack>
                        </Paper>
                    ))}
                </Stack>
            </DialogContent>
            <DialogActions>
                <Button onClick={onClose} color="inherit">Close</Button>
            </DialogActions>
        </Dialog>
    );
}

