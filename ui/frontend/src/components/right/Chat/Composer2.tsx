import React from 'react';
import { 
    Box, 
    TextField, 
    IconButton, 
    Chip, 
    FormControl, 
    Select, 
    MenuItem, 
    Tooltip,
    Paper,
    Stack,
    Typography,
    Fade
} from '@mui/material';
import { 
    SendRounded, 
    StopRounded, 
    AttachFileRounded, 
    CloseRounded, 
    PersonRounded,
    DragIndicatorRounded
} from '@mui/icons-material';

type Props = {
    input: string;
    setInput: (val: string) => void;
    busy: boolean;
    onSend: () => void;
    onStop?: () => void;
    onClear: () => void;
    // When this number changes, focus the input
    focusToken?: number;
    attachments?: string[];
    onRemoveAttachment?: (path: string) => void;
    onOpenAttach?: (anchorEl: HTMLElement | null) => void;
    onAttachButtonRef?: (el: HTMLElement | null) => void;
    // Personality integration
    currentPersonality?: string;
    setCurrentPersonality?: (personality: string) => void;
    personalities?: Record<string, { name: string; description: string; prompt: string }>;
};

function Composer2Component({ 
    input, 
    setInput, 
    busy, 
    onSend, 
    onStop, 
    focusToken, 
    attachments = [], 
    onRemoveAttachment, 
    onOpenAttach, 
    onAttachButtonRef,
    currentPersonality = 'coder',
    setCurrentPersonality,
    personalities = {}
}: Props) {
    const inputRef = React.useRef<HTMLInputElement | HTMLTextAreaElement | null>(null);
    const attachBtnRef = React.useRef<HTMLButtonElement | null>(null);
    const [isDragOver, setIsDragOver] = React.useState(false);
    const [showPersonalityTooltip, setShowPersonalityTooltip] = React.useState(false);

    React.useEffect(() => {
        if (focusToken === undefined) return;
        try {
            inputRef.current?.focus();
        } catch { }
    }, [focusToken]);

    React.useEffect(() => {
        if (onAttachButtonRef) {
            onAttachButtonRef(attachBtnRef.current);
        }
    }, [onAttachButtonRef]);

    // Handle file drop
    const handleDrop = React.useCallback((e: React.DragEvent) => {
        e.preventDefault();
        setIsDragOver(false);
        
        const files = Array.from(e.dataTransfer.files);
        // For now, just add file names as attachments
        // In a real implementation, you'd handle file upload/processing
        files.forEach(file => {
            if (onRemoveAttachment && !attachments.includes(file.name)) {
                // This is a bit of a hack since we don't have an addAttachment prop
                // In practice, you'd want to add a proper file handling mechanism
            }
        });
    }, [attachments, onRemoveAttachment]);

    const handleDragOver = React.useCallback((e: React.DragEvent) => {
        e.preventDefault();
        setIsDragOver(true);
    }, []);

    const handleDragLeave = React.useCallback((e: React.DragEvent) => {
        e.preventDefault();
        setIsDragOver(false);
    }, []);

    const hasContent = input.trim() || attachments.length > 0;
    const personalityConfig = personalities[currentPersonality];

    return (
        <Paper
            elevation={3}
            onDrop={handleDrop}
            onDragOver={handleDragOver}
            onDragLeave={handleDragLeave}
            sx={{
                position: 'relative',
                borderRadius: 3,
                background: 'linear-gradient(135deg, rgba(255,255,255,0.02) 0%, rgba(255,255,255,0.01) 100%)',
                backdropFilter: 'blur(20px)',
                border: '1px solid',
                borderColor: isDragOver ? 'primary.main' : 'divider',
                transition: 'all 0.2s cubic-bezier(0.4, 0, 0.2, 1)',
                '&:hover': {
                    borderColor: 'primary.main',
                    boxShadow: '0 8px 32px rgba(0,0,0,0.12)',
                },
                '&:focus-within': {
                    borderColor: 'primary.main',
                    boxShadow: '0 0 0 2px rgba(25, 118, 210, 0.12)',
                },
                ...(isDragOver && {
                    borderColor: 'primary.main',
                    backgroundColor: 'rgba(25, 118, 210, 0.04)',
                })
            }}
        >
            {isDragOver && (
                <Fade in={isDragOver}>
                    <Box
                        sx={{
                            position: 'absolute',
                            inset: 0,
                            display: 'flex',
                            alignItems: 'center',
                            justifyContent: 'center',
                            borderRadius: 3,
                            backgroundColor: 'rgba(25, 118, 210, 0.08)',
                            border: '2px dashed',
                            borderColor: 'primary.main',
                            zIndex: 10,
                        }}
                    >
                        <Stack alignItems="center" spacing={1}>
                            <DragIndicatorRounded sx={{ fontSize: 48, color: 'primary.main', opacity: 0.7 }} />
                            <Typography variant="h6" color="primary.main" fontWeight={600}>
                                Drop files to attach
                            </Typography>
                        </Stack>
                    </Box>
                </Fade>
            )}

            <Box sx={{ p: 2 }}>
                {/* Top row: Personality selector and chips */}
                <Stack direction="row" spacing={1} sx={{ mb: hasContent ? 1.5 : 0 }}>
                    {/* Personality Selector */}
                    {setCurrentPersonality && (
                        <Tooltip 
                            title={personalityConfig ? personalityConfig.description : 'Select AI personality'} 
                            placement="top"
                            open={showPersonalityTooltip}
                            onOpen={() => setShowPersonalityTooltip(true)}
                            onClose={() => setShowPersonalityTooltip(false)}
                        >
                            <FormControl size="small" sx={{ minWidth: 120 }}>
                                <Select
                                    value={currentPersonality}
                                    onChange={(e) => setCurrentPersonality(e.target.value)}
                                    displayEmpty
                                    variant="outlined"
                                    startAdornment={<PersonRounded sx={{ mr: 0.5, fontSize: 18 }} />}
                                    sx={{
                                        height: 32,
                                        borderRadius: 2,
                                        '& .MuiSelect-select': {
                                            py: 0.5,
                                            display: 'flex',
                                            alignItems: 'center',
                                        },
                                        '& .MuiOutlinedInput-notchedOutline': {
                                            borderColor: 'rgba(255,255,255,0.1)',
                                        },
                                        '&:hover .MuiOutlinedInput-notchedOutline': {
                                            borderColor: 'primary.main',
                                        },
                                    }}
                                >
                                    {[
                                        'coder',      // The Coder (default)
                                        'architect',  // The Architect
                                        'debugger',   // The Debugger
                                        'reviewer',   // The Reviewer
                                        'founder',    // The Founder
                                        'scientist',  // Mad Scientist
                                        'comedian',   // Stand-up Comedian
                                        'pirate',     // Pirate Captain
                                        'bavarian',   // The Bavarian Boy
                                        'waifu',      // Anime Waifu
                                    ]
                                        .filter(key => personalities[key])
                                        .map((key) => {
                                            const config = personalities[key];
                                            return (
                                                <MenuItem key={key} value={key}>
                                                    <Typography variant="body2" fontWeight={500}>
                                                        {config.name}
                                                    </Typography>
                                                </MenuItem>
                                            );
                                        })}
                                </Select>
                            </FormControl>
                        </Tooltip>
                    )}

                    {/* Attachment Chips */}
                    {attachments.map((attachment, index) => (
                        <Chip
                            key={`${attachment}-${index}`}
                            size="small"
                            label={attachment.split('/').pop() || attachment}
                            onDelete={onRemoveAttachment ? () => onRemoveAttachment(attachment) : undefined}
                            deleteIcon={<CloseRounded fontSize="small" />}
                            variant="outlined"
                            sx={{
                                maxWidth: 200,
                                height: 32,
                                borderRadius: 2,
                                backgroundColor: 'rgba(25, 118, 210, 0.08)',
                                borderColor: 'primary.main',
                                color: 'primary.main',
                                '& .MuiChip-label': {
                                    px: 1.5,
                                    fontWeight: 500,
                                    fontSize: '0.75rem',
                                },
                                '& .MuiChip-deleteIcon': {
                                    color: 'primary.main',
                                    '&:hover': {
                                        color: 'primary.dark',
                                    }
                                }
                            }}
                        />
                    ))}
                </Stack>

                {/* Text Input Container */}
                <Box sx={{ position: 'relative' }}>
                    <TextField
                        value={input}
                        onChange={(e) => setInput(e.target.value)}
                        inputRef={inputRef as any}
                        onKeyDown={(e) => {
                            if ((e as any).key === 'Enter' && !(e as any).shiftKey) {
                                e.preventDefault();
                                if (!busy && input.trim()) onSend();
                            }
                        }}
                        placeholder="Ask Loom anything…"
                        disabled={busy}
                        multiline
                        minRows={1}
                        maxRows={12}
                        fullWidth
                        variant="outlined"
                        sx={{
                            '& .MuiOutlinedInput-root': {
                                borderRadius: 2,
                                backgroundColor: 'transparent',
                                pr: 10, // Space for buttons
                                '& fieldset': {
                                    border: 'none',
                                },
                                '& textarea': {
                                    resize: 'none',
                                    scrollbarWidth: 'thin',
                                    '&::-webkit-scrollbar': {
                                        width: 6,
                                    },
                                    '&::-webkit-scrollbar-track': {
                                        background: 'transparent',
                                    },
                                    '&::-webkit-scrollbar-thumb': {
                                        background: 'rgba(255,255,255,0.2)',
                                        borderRadius: 3,
                                    },
                                    '&::-webkit-scrollbar-thumb:hover': {
                                        background: 'rgba(255,255,255,0.3)',
                                    },
                                }
                            },
                            '& .MuiInputBase-input': {
                                fontSize: '0.95rem',
                                lineHeight: 1.5,
                                '&::placeholder': {
                                    color: 'text.secondary',
                                    opacity: 0.7,
                                }
                            }
                        }}
                    />

                    {/* Action Buttons */}
                    <Box
                        sx={{
                            position: 'absolute',
                            right: 8,
                            bottom: 8,
                            display: 'flex',
                            gap: 0.5,
                        }}
                    >
                        {/* Attachment Button */}
                        <Tooltip title="Attach files" placement="top">
                            <IconButton
                                ref={attachBtnRef}
                                onClick={(e) => onOpenAttach?.(e.currentTarget)}
                                disabled={busy}
                                size="small"
                                sx={{
                                    width: 36,
                                    height: 36,
                                    backgroundColor: 'rgba(255,255,255,0.05)',
                                    backdropFilter: 'blur(8px)',
                                    border: '1px solid rgba(255,255,255,0.1)',
                                    color: 'text.secondary',
                                    transition: 'all 0.2s cubic-bezier(0.4, 0, 0.2, 1)',
                                    '&:hover': {
                                        backgroundColor: 'rgba(255,255,255,0.08)',
                                        borderColor: 'primary.main',
                                        color: 'primary.main',
                                        transform: 'translateY(-1px)',
                                    },
                                    '&:disabled': {
                                        opacity: 0.5,
                                        transform: 'none',
                                    }
                                }}
                            >
                                <AttachFileRounded fontSize="small" />
                            </IconButton>
                        </Tooltip>

                        {/* Send/Stop Button */}
                        <Tooltip title={busy ? 'Stop generation' : 'Send message'} placement="top">
                            <IconButton
                                onClick={busy ? onStop : onSend}
                                disabled={busy ? false : !input.trim()}
                                size="small"
                                sx={{
                                    width: 36,
                                    height: 36,
                                    backgroundColor: busy ? 'error.main' : 'primary.main',
                                    color: busy ? 'error.contrastText' : 'primary.contrastText',
                                    transition: 'all 0.2s cubic-bezier(0.4, 0, 0.2, 1)',
                                    '&:hover': {
                                        backgroundColor: busy ? 'error.dark' : 'primary.dark',
                                        transform: 'translateY(-1px)',
                                        boxShadow: '0 4px 12px rgba(0,0,0,0.2)',
                                    },
                                    '&:disabled': {
                                        opacity: 0.4,
                                        backgroundColor: 'action.disabled',
                                        color: 'text.disabled',
                                        transform: 'none',
                                    },
                                    '&:active': {
                                        transform: 'translateY(0px)',
                                    }
                                }}
                            >
                                {busy ? <StopRounded fontSize="small" /> : <SendRounded fontSize="small" />}
                            </IconButton>
                        </Tooltip>
                    </Box>
                </Box>
            </Box>
        </Paper>
    );
}

export default React.memo(Composer2Component, (prev, next) => {
    return (
        prev.input === next.input &&
        prev.busy === next.busy &&
        prev.onSend === next.onSend &&
        prev.onStop === next.onStop &&
        prev.focusToken === next.focusToken &&
        prev.attachments === next.attachments &&
        prev.currentPersonality === next.currentPersonality &&
        prev.personalities === next.personalities
    );
});
