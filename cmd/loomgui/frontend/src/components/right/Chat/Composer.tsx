import { Stack, TextField, Button } from '@mui/material';
import SendIcon from '@mui/icons-material/Send';

type Props = {
    input: string;
    setInput: (val: string) => void;
    busy: boolean;
    onSend: () => void;
    onClear: () => void;
};

export default function Composer({ input, setInput, busy, onSend, onClear }: Props) {
    return (
        <Stack direction="row" spacing={1} alignItems="flex-end">
            <TextField
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onKeyDown={(e) => {
                    if ((e as any).key === 'Enter' && !(e as any).shiftKey) {
                        e.preventDefault();
                        if (!busy) onSend();
                    }
                }}
                placeholder="Ask Loom anything…"
                disabled={busy}
                multiline
                minRows={1}
                maxRows={8}
                fullWidth
            />
            <Button onClick={onClear} color="inherit" disabled={busy} variant="outlined">
                Clear
            </Button>
            <Button onClick={onSend} disabled={busy || !input.trim()} variant="contained" endIcon={<SendIcon />}>
                {busy ? 'Working…' : 'Send'}
            </Button>
        </Stack>
    );
}


