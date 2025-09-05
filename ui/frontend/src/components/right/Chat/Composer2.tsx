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
import ModelSelector from '@/ModelSelector';

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
    // Model integration
    currentModel?: string;
    onSelectModel?: (model: string) => void;
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
    personalities = {},
    currentModel = '',
    onSelectModel
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

    const personalityConfig = personalities[currentPersonality];

    return (
        <Paper
            elevation={2}
            onDrop={handleDrop}
            onDragOver={handleDragOver}
            onDragLeave={handleDragLeave}
            sx={{
                position: 'relative',
                borderRadius: 1.5,
                background: 'linear-gradient(135deg, rgba(255,255,255,0.02) 0%, rgba(255,255,255,0.01) 100%)',
                backdropFilter: 'blur(20px)',
                border: '1px solid',
                borderColor: isDragOver ? 'primary.main' : 'divider',
                transition: 'all 0.2s cubic-bezier(0.4, 0, 0.2, 1)',
                '&:hover': {
                    borderColor: 'primary.main',
                    boxShadow: '0 4px 16px rgba(0,0,0,0.08)',
                },
                '&:focus-within': {
                    borderColor: 'primary.main',
                    boxShadow: '0 0 0 1px rgba(25, 118, 210, 0.12)',
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
                            borderRadius: 1.5,
                            backgroundColor: 'rgba(25, 118, 210, 0.08)',
                            border: '2px dashed',
                            borderColor: 'primary.main',
                            zIndex: 10,
                        }}
                    >
                        <Stack alignItems="center" spacing={0.5}>
                            <DragIndicatorRounded sx={{ fontSize: 32, color: 'primary.main', opacity: 0.7 }} />
                            <Typography variant="body2" color="primary.main" fontWeight={500}>
                                Drop files to attach
                            </Typography>
                        </Stack>
                    </Box>
                </Fade>
            )}

            <Box sx={{ p: 1.25 }}>
                {/* Top row: Personality selector and Attachment Chips */}
                {(setCurrentPersonality || attachments.length > 0) && (
                    <Stack direction="row" spacing={0.75} sx={{ mb: 0.75, flexWrap: 'wrap', alignItems: 'center' }}>
                        {/* Personality Selector */}
                        {setCurrentPersonality && (
                            <FormControl size="small" sx={{ minWidth: 100 }}>
                                <Select
                                    value={currentPersonality}
                                    onChange={(e) => setCurrentPersonality(e.target.value)}
                                    displayEmpty
                                    variant="outlined"
                                    startAdornment={<PersonRounded sx={{ mr: 0.5, fontSize: 16 }} />}
                                    renderValue={(val) =>
                                        val ? personalities[val as string]?.name ?? '' : ''
                                    }
                                    MenuProps={{
                                        PaperProps: { sx: { mt: 0.5 } },
                                    }}
                                    sx={{
                                        height: 36,
                                        borderRadius: 1.5,
                                        fontSize: '0.875rem',
                                        backgroundColor: 'rgba(255,255,255,0.03)',
                                        backdropFilter: 'blur(8px)',
                                        border: '1px solid transparent',
                                        '& .MuiSelect-select': { py: 0.5, px: 1, display: 'flex', alignItems: 'center' },
                                        '& .MuiOutlinedInput-notchedOutline': { border: 'none' },
                                        '&:hover .MuiOutlinedInput-notchedOutline': { border: 'none' },
                                        '&.Mui-focused .MuiOutlinedInput-notchedOutline': { border: 'none' },
                                        '&:hover': { backgroundColor: 'rgba(255,255,255,0.06)', borderColor: 'primary.main' },
                                        '&.Mui-focused': {
                                            borderColor: 'primary.main',
                                            backgroundColor: 'primary.main',
                                            color: 'primary.contrastText',
                                            '& .MuiSvgIcon-root': { color: 'primary.contrastText' },
                                        },
                                    }}
                                >
                                    {[
                                        'coder',
                                        'architect',
                                        'debugger',
                                        'reviewer',
                                        'founder',
                                        'scientist',
                                        'comedian',
                                        'pirate',
                                        'bavarian',
                                        'waifu',
                                    ]
                                        .filter((key) => personalities[key])
                                        .map((key) => {
                                            const config = personalities[key];
                                            return (
                                                <MenuItem key={key} value={key}>
                                                    <Tooltip
                                                        title={
                                                            <Box>
                                                                <Typography variant="subtitle1" fontWeight="bold">
                                                                    {config.name}
                                                                </Typography>
                                                                <Typography variant="body2" sx={{ mt: 0.5 }}>
                                                                    {config.description}
                                                                </Typography>
                                                            </Box>
                                                        }
                                                        placement="right"
                                                        arrow
                                                        enterDelay={0}
                                                    >
                                                        <Box sx={{ display: 'flex', alignItems: 'center', width: '100%' }}>
                                                            <Typography variant="body2" fontWeight={500}>
                                                                {config.name}
                                                            </Typography>
                                                        </Box>
                                                    </Tooltip>
                                                </MenuItem>
                                            );
                                        })}
                                </Select>
                            </FormControl>
                        )}

                        {/* Attachment Chips */}
                        {attachments.map((attachment, index) => (
                            <Chip
                                key={`${attachment}-${index}`}
                                size="small"
                                label={attachment.split('/').pop() || attachment}
                                onDelete={onRemoveAttachment ? () => onRemoveAttachment(attachment) : undefined}
                                deleteIcon={<CloseRounded fontSize="inherit" />}
                                variant="outlined"
                                sx={{
                                    maxWidth: 140,
                                    height: 24,
                                    borderRadius: 1.5,
                                    backgroundColor: 'rgba(25, 118, 210, 0.08)',
                                    borderColor: 'primary.main',
                                    color: 'primary.main',
                                    '& .MuiChip-label': {
                                        px: 1,
                                        fontWeight: 500,
                                        fontSize: '0.65rem',
                                    },
                                    '& .MuiChip-deleteIcon': {
                                        fontSize: '0.875rem',
                                        color: 'primary.main',
                                        '&:hover': {
                                            color: 'primary.dark',
                                        }
                                    }
                                }}
                            />
                        ))}
                    </Stack>
                )}

                {/* Text Input Container */}
                <Box sx={{ mb: 0.75 }}>
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
                                borderRadius: 1.5,
                                backgroundColor: 'transparent',
                                transition: 'all 0.2s ease-in-out',
                                '& fieldset': {
                                    border: 'none',
                                },
                                '&.Mui-focused': {
                                    boxShadow: '0 0 0 1px rgba(var(--mui-palette-primary-mainChannel) / 0.2)',
                                },
                                '& textarea': {
                                    resize: 'none',
                                    scrollbarWidth: 'thin',
                                    '&::-webkit-scrollbar': {
                                        width: 4,
                                    },
                                    '&::-webkit-scrollbar-track': {
                                        background: 'transparent',
                                    },
                                    '&::-webkit-scrollbar-thumb': {
                                        background: 'rgba(255,255,255,0.2)',
                                        borderRadius: 2,
                                    },
                                    '&::-webkit-scrollbar-thumb:hover': {
                                        background: 'rgba(255,255,255,0.3)',
                                    },
                                }
                            },
                            '& .MuiInputBase-input': {
                                fontSize: '0.875rem',
                                lineHeight: 1.4,
                                py: 0,
                                '&::placeholder': {
                                    color: 'text.secondary',
                                    opacity: 0.7,
                                }
                            }
                        }}
                    />
                </Box>

                {/* Bottom row: Model selector and Action buttons */}
                <Stack direction="row" spacing={0.75} alignItems="center">
                    {/* Model Selector */}
                    {onSelectModel && (
                        <Box sx={{
                            flex: 1,
                            minWidth: 220,
                            maxWidth: 320,
                            '& .MuiAutocomplete-root': {
                                width: '100% !important',
                            },
                            '& .MuiTextField-root': {
                                width: '100% !important',
                                minWidth: 'unset !important',
                                maxWidth: 'unset !important',
                            },
                            '& .MuiInputBase-root': {
                                height: 36,
                                fontSize: '0.875rem',
                                pr: '12px !important', // Less space needed without clear button
                            },
                            '& .MuiAutocomplete-endAdornment': {
                                display: 'none', // Hide the endAdornment completely
                            },
                            '& .MuiAutocomplete-input': {
                                pr: '0 !important',
                                minWidth: '0 !important',
                            }
                        }}>
                            <ModelSelector
                                onSelect={onSelectModel}
                                currentModel={currentModel}
                            />
                        </Box>
                    )}

                    {/* Spacer to push buttons to the right */}
                    <Box sx={{ flex: 1 }} />

                    {/* Action Buttons */}
                    <Stack direction="row" spacing={0.25}>
                        {/* Attachment Button */}
                        <Tooltip title="Attach files" placement="top">
                            <IconButton
                                ref={attachBtnRef}
                                onClick={(e) => onOpenAttach?.(e.currentTarget)}
                                disabled={busy}
                                size="small"
                                sx={{
                                    width: 32,
                                    height: 32,
                                    backgroundColor: 'rgba(255,255,255,0.05)',
                                    backdropFilter: 'blur(8px)',
                                    border: '1px solid rgba(255,255,255,0.1)',
                                    color: 'text.secondary',
                                    transition: 'all 0.2s cubic-bezier(0.4, 0, 0.2, 1)',
                                    '&:hover': {
                                        backgroundColor: 'primary.main',
                                        borderColor: 'primary.main',
                                        color: 'primary.contrastText',
                                        transform: 'translateY(-0.5px)',
                                        boxShadow: '0 2px 8px rgba(0,0,0,0.15)',
                                    },
                                    '&:disabled': {
                                        opacity: 0.5,
                                        transform: 'none',
                                    }
                                }}
                            >
                                <AttachFileRounded sx={{ fontSize: 18 }} />
                            </IconButton>
                        </Tooltip>

                        {/* Send/Stop Button */}
                        <Tooltip title={busy ? 'Stop generation' : 'Send message'} placement="top">
                            <IconButton
                                onClick={busy ? onStop : onSend}
                                disabled={busy ? false : !input.trim()}
                                size="small"
                                sx={{
                                    width: 32,
                                    height: 32,
                                    backgroundColor: busy ? 'error.main' : 'primary.main',
                                    color: busy ? 'error.contrastText' : 'primary.contrastText',
                                    transition: 'all 0.2s cubic-bezier(0.4, 0, 0.2, 1)',
                                    '&:hover': {
                                        backgroundColor: busy ? 'error.dark' : 'primary.dark',
                                        transform: 'translateY(-0.5px)',
                                        boxShadow: '0 2px 8px rgba(0,0,0,0.15)',
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
                                {busy ? <StopRounded sx={{ fontSize: 18 }} /> : <SendRounded sx={{ fontSize: 18 }} />}
                            </IconButton>
                        </Tooltip>
                    </Stack>
                </Stack>
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
        prev.personalities === next.personalities &&
        prev.currentModel === next.currentModel &&
        prev.onSelectModel === next.onSelectModel
    );
});
