import React from 'react';
import { Stack, TextField, IconButton } from '@mui/material';
import { PendingRounded, SendRounded } from '@mui/icons-material';

type Props = {
    input: string;
    setInput: (val: string) => void;
    busy: boolean;
    onSend: () => void;
    onClear: () => void;
    // When this number changes, focus the input
    focusToken?: number;
};

function ComposerComponent({ input, setInput, busy, onSend, focusToken }: Props) {
    const inputRef = React.useRef<HTMLInputElement | HTMLTextAreaElement | null>(null);

    React.useEffect(() => {
        if (focusToken === undefined) return;
        try {
            inputRef.current?.focus();
        } catch {}
    }, [focusToken]);
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
                inputRef={inputRef as any}
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

export default React.memo(ComposerComponent, (prev, next) => {
    return (
        prev.input === next.input &&
        prev.busy === next.busy &&
        prev.onSend === next.onSend &&
        prev.focusToken === next.focusToken
    );
});


