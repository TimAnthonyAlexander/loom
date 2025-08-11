import React from 'react';
import { Stack, TextField, IconButton, Chip, Tooltip, Box } from '@mui/material';
import { PendingRounded, SendRounded, AttachFileRounded, CloseRounded } from '@mui/icons-material';

type Props = {
    input: string;
    setInput: (val: string) => void;
    busy: boolean;
    onSend: () => void;
    onClear: () => void;
    // When this number changes, focus the input
    focusToken?: number;
    attachments?: string[];
    onRemoveAttachment?: (path: string) => void;
    onOpenAttach?: () => void;
};

function ComposerComponent({ input, setInput, busy, onSend, focusToken, attachments = [], onRemoveAttachment, onOpenAttach }: Props) {
    const inputRef = React.useRef<HTMLInputElement | HTMLTextAreaElement | null>(null);

    React.useEffect(() => {
        if (focusToken === undefined) return;
        try {
            inputRef.current?.focus();
        } catch { }
    }, [focusToken]);
    return (
        <Box>
            {!!attachments.length && (
                <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5, mb: 1 }}>
                    {attachments.map((p) => (
                        <Chip
                            key={p}
                            size="small"
                            label={p}
                            onDelete={onRemoveAttachment ? () => onRemoveAttachment(p) : undefined}
                            deleteIcon={<CloseRounded fontSize="small" />}
                            sx={{ maxWidth: '100%' }}
                        />
                    ))}
                </Box>
            )}
            <Stack
                direction="row"
                spacing={1}
                alignItems="flex-end"
                sx={{ position: 'relative' }}
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
                <Tooltip title="Attach files (Cmd/Ctrl+Shift+P)" placement="left">
                    <span>
                        <IconButton onClick={onOpenAttach} disabled={busy} sx={{ position: 'absolute', bottom: 5, right: 44 }}>
                            <AttachFileRounded />
                        </IconButton>
                    </span>
                </Tooltip>
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
                    {busy ? <PendingRounded /> : <SendRounded />}
                </IconButton>
            </Stack>
        </Box>
    );
}

export default React.memo(ComposerComponent, (prev, next) => {
    return (
        prev.input === next.input &&
        prev.busy === next.busy &&
        prev.onSend === next.onSend &&
        prev.focusToken === next.focusToken &&
        prev.attachments === next.attachments
    );
});


