import { Stack, TextField, IconButton } from '@mui/material';
import { PendingRounded, SendRounded } from '@mui/icons-material';

type Props = {
    input: string;
    setInput: (val: string) => void;
    busy: boolean;
    onSend: () => void;
    onClear: () => void;
};

export default function Composer({ input, setInput, busy, onSend }: Props) {
    return (
        <Stack
            direction="row"
            spacing={1}
            alignItems="flex-end"
            sx={{
                position: 'relative',
            }}
        >
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
            <IconButton
                onClick={onSend}
                disabled={busy || !input.trim()}
                sx={{
                    position: 'absolute',
                    height: '100%',
                    bottom: 5,
                    right: 5,
                }}
            >
                {busy ? (
                    <PendingRounded
                    />
                ) : (
                    <SendRounded
                    />
                )}
            </IconButton>
        </Stack >
    );
}


