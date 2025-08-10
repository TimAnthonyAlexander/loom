import { useEffect, useMemo, useState } from 'react';
import {
    Dialog,
    DialogTitle,
    DialogContent,
    Box,
    TextField,
    Tabs,
    Tab,
    List,
    ListItemButton,
    ListItemText,
    InputAdornment,
    IconButton,
    Typography,
} from '@mui/material';
import SearchIcon from '@mui/icons-material/Search';
import CloseIcon from '@mui/icons-material/Close';
import * as AppBridge from '../../../wailsjs/go/bridge/App';

type Mode = 'files' | 'text';

type Props = {
    open: boolean;
    initialMode?: Mode;
    onClose: () => void;
    onOpenFile: (path: string, line?: number, column?: number) => void;
};

type TextMatch = {
    path: string;
    line_number: number;
    line_text: string;
    start_char?: number;
    end_char?: number;
};

export default function SearchDialog({ open, initialMode = 'files', onClose, onOpenFile }: Props) {
    const [mode, setMode] = useState<Mode>(initialMode);
    const [query, setQuery] = useState('');
    const [glob, setGlob] = useState('');
    const [subdir, setSubdir] = useState('');
    const [fileResults, setFileResults] = useState<string[]>([]);
    const [textResults, setTextResults] = useState<TextMatch[]>([]);
    const [selectedIndex, setSelectedIndex] = useState(0);

    useEffect(() => {
        if (!open) return;
        setMode(initialMode);
        setSelectedIndex(0);
        // Focus handled by autoFocus on input
    }, [open, initialMode]);

    // Debounce helper
    const useDebounced = (value: string, delayMs: number) => {
        const [debounced, setDebounced] = useState(value);
        useEffect(() => {
            const t = setTimeout(() => setDebounced(value), delayMs);
            return () => clearTimeout(t);
        }, [value, delayMs]);
        return debounced;
    };

    const debouncedQuery = useDebounced(query, 150);
    const debouncedGlob = useDebounced(glob, 150);
    const debouncedSubdir = useDebounced(subdir, 150);

    useEffect(() => {
        if (!open) return;
        if (mode === 'files') {
            // Quick open by file name/pattern
            if (!debouncedQuery && !debouncedGlob) {
                setFileResults([]);
                return;
            }
            AppBridge.FindFiles(debouncedGlob || debouncedQuery, debouncedSubdir, 200)
                .then((list: any) => {
                    const arr = Array.isArray(list) ? list as string[] : [];
                    setFileResults(arr);
                    setSelectedIndex(0);
                })
                .catch(() => setFileResults([]));
        } else {
            // Content search
            if (!debouncedQuery) {
                setTextResults([]);
                return;
            }
            AppBridge.SearchCode(debouncedQuery, debouncedGlob || '', 200)
                .then((matches: any) => {
                    const arr = Array.isArray(matches) ? (matches as TextMatch[]) : [];
                    setTextResults(arr);
                    setSelectedIndex(0);
                })
                .catch(() => setTextResults([]));
        }
    }, [open, mode, debouncedQuery, debouncedGlob, debouncedSubdir]);

    const handleEnter = () => {
        if (mode === 'files') {
            const item = fileResults[selectedIndex];
            if (item) onOpenFile(item);
        } else {
            const item = textResults[selectedIndex];
            if (item) onOpenFile(item.path, item.line_number, (item.start_char || 0) + 1);
        }
    };

    const resultsCount = useMemo(() => (mode === 'files' ? fileResults.length : textResults.length), [mode, fileResults, textResults]);

    return (
        <Dialog open={open} onClose={onClose} fullWidth maxWidth="md">
            <DialogTitle sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                <Tabs value={mode} onChange={(_, v) => setMode(v)} sx={{ minHeight: 36 }}>
                    <Tab label="Files" value="files" sx={{ minHeight: 36 }} />
                    <Tab label="Text" value="text" sx={{ minHeight: 36 }} />
                </Tabs>
                <Box sx={{ flex: 1 }} />
                <IconButton onClick={onClose} size="small"><CloseIcon fontSize="small" /></IconButton>
            </DialogTitle>
            <DialogContent>
                <Box sx={{ display: 'flex', gap: 1, alignItems: 'center', mb: 1 }}>
                    <TextField
                        autoFocus
                        size="small"
                        value={query}
                        onChange={(e) => setQuery(e.target.value)}
                        onKeyDown={(e) => {
                            if (e.key === 'Enter') { e.preventDefault(); handleEnter(); }
                            if (e.key === 'ArrowDown') { e.preventDefault(); setSelectedIndex((i) => Math.min(resultsCount - 1, i + 1)); }
                            if (e.key === 'ArrowUp') { e.preventDefault(); setSelectedIndex((i) => Math.max(0, i - 1)); }
                        }}
                        placeholder={mode === 'files' ? 'Search files (name or glob)…' : 'Search text…'}
                        fullWidth
                        InputProps={{ startAdornment: (<InputAdornment position="start"><SearchIcon fontSize="small" /></InputAdornment>) }}
                    />
                    <TextField
                        size="small"
                        value={glob}
                        onChange={(e) => setGlob(e.target.value)}
                        placeholder={mode === 'files' ? 'Filter (e.g., *.ts, src/**)' : 'File filter (glob)'}
                        sx={{ width: 240 }}
                    />
                    {mode === 'files' && (
                        <TextField
                            size="small"
                            value={subdir}
                            onChange={(e) => setSubdir(e.target.value)}
                            placeholder="Within folder (optional)"
                            sx={{ width: 220 }}
                        />
                    )}
                </Box>
                {mode === 'files' ? (
                    <List dense sx={{ maxHeight: 420, overflowY: 'auto' }}>
                        {fileResults.map((p, idx) => (
                            <ListItemButton
                                key={p}
                                selected={idx === selectedIndex}
                                onClick={() => onOpenFile(p)}
                                onMouseEnter={() => setSelectedIndex(idx)}
                            >
                                <ListItemText primaryTypographyProps={{ fontFamily: 'ui-monospace, Menlo, monospace', fontSize: 13 }} primary={p} />
                            </ListItemButton>
                        ))}
                        {fileResults.length === 0 && (
                            <Box sx={{ p: 2 }}>
                                <Typography variant="body2" color="text.secondary">No files.</Typography>
                            </Box>
                        )}
                    </List>
                ) : (
                    <List dense sx={{ maxHeight: 420, overflowY: 'auto' }}>
                        {textResults.map((m, idx) => (
                            <ListItemButton
                                key={`${m.path}:${m.line_number}:${idx}`}
                                selected={idx === selectedIndex}
                                onClick={() => onOpenFile(m.path, m.line_number, (m.start_char || 0) + 1)}
                                onMouseEnter={() => setSelectedIndex(idx)}
                            >
                                <ListItemText
                                    primaryTypographyProps={{ fontFamily: 'ui-monospace, Menlo, monospace', fontSize: 13 }}
                                    secondaryTypographyProps={{ fontFamily: 'ui-monospace, Menlo, monospace', fontSize: 12 }}
                                    primary={`${m.path}:${m.line_number}`}
                                    secondary={m.line_text}
                                />
                            </ListItemButton>
                        ))}
                        {textResults.length === 0 && (
                            <Box sx={{ p: 2 }}>
                                <Typography variant="body2" color="text.secondary">No matches.</Typography>
                            </Box>
                        )}
                    </List>
                )}
            </DialogContent>
        </Dialog>
    );
}


