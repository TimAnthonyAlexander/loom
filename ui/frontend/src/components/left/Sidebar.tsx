import React from 'react';
import { Box, Typography, IconButton, Tooltip, Paper, LinearProgress } from '@mui/material';
import SettingsIcon from '@mui/icons-material/Settings';
import RuleIcon from '@mui/icons-material/Rule';
import MemoryIcon from '@mui/icons-material/BookmarkBorder';
import FileExplorer from './Files/FileExplorer';
import { UIFileEntry } from '../../types/ui';

type SidebarProps = {
    onOpenWorkspace: () => void;
    onOpenRules: () => void;
    onOpenMemories?: () => void;
    onOpenSettings: () => void;
    onOpenCosts: () => void;
    totalInUSD: number;
    totalOutUSD: number;
    dirCache: Record<string, UIFileEntry[]>;
    expandedDirs: Record<string, boolean>;
    onToggleDir: (path: string) => void;
    onOpenFile: (path: string) => void;
    indexing?: { status: 'idle' | 'start' | 'progress' | 'done'; total: number; done: number; file: string };
};

function Sidebar(props: SidebarProps) {
    const {
        onOpenWorkspace,
        onOpenRules,
        onOpenSettings,
        onOpenMemories,
        onOpenCosts,
        totalInUSD,
        totalOutUSD,
        dirCache,
        expandedDirs,
        onToggleDir,
        onOpenFile,
        indexing,
    } = props;

    return (
        <Box
            sx={{
                px: 1,
                py: 1,
                display: 'flex',
                flexDirection: 'column',
                gap: 2,
                overflowY: 'auto',
                height: '100%',
            }}
        >
            <Box sx={{ pt: 4 }}>
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.5 }}>
                    <Box component="img" src="/logo.png" alt="Loom" sx={{ width: 28, height: 28, borderRadius: 0.5 }} />
                    <Typography variant="h6" fontWeight={600}>
                        Loom
                    </Typography>
                    <Box sx={{ flex: 1 }} />
                    <Tooltip title="Select Workspace">
                        <IconButton size="small" onClick={onOpenWorkspace}>
                            <Typography variant="caption">WS</Typography>
                        </IconButton>
                    </Tooltip>
                    <Tooltip title="Rules">
                        <IconButton size="small" onClick={onOpenRules}>
                            <RuleIcon fontSize="small" />
                        </IconButton>
                    </Tooltip>
                    <Tooltip title="Memories">
                        <IconButton size="small" onClick={onOpenMemories}>
                            <MemoryIcon fontSize="small" />
                        </IconButton>
                    </Tooltip>
                    <Tooltip title="Settings">
                        <IconButton size="small" onClick={onOpenSettings}>
                            <SettingsIcon fontSize="small" />
                        </IconButton>
                    </Tooltip>
                </Box>
                {indexing && (indexing.status === 'start' || indexing.status === 'progress') && (
                    <Box sx={{ mt: 1 }}>
                        <Typography variant="caption" color="text.secondary">
                            Indexing symbols… {Math.min(100, Math.floor((indexing.done / Math.max(1, indexing.total)) * 100))}%
                        </Typography>
                        <LinearProgress variant="determinate" value={Math.min(100, (indexing.done / Math.max(1, indexing.total)) * 100)} sx={{ height: 4, borderRadius: 1, mt: 0.5 }} />
                        {indexing.file && (
                            <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 0.25 }} noWrap>
                                {indexing.file}
                            </Typography>
                        )}
                    </Box>
                )}
            </Box>

            <Box
                sx={{
                    flex: 1,
                    minHeight: 0,
                    display: 'flex',
                    flexDirection: 'column',
                }}
            >
                <Paper
                    variant="outlined"
                    sx={{
                        flex: 1,
                        overflowY: 'auto',
                        display: 'flex',
                        flexDirection: 'column',
                    }}
                >
                    <FileExplorer
                        dirCache={dirCache}
                        expandedDirs={expandedDirs}
                        onToggleDir={onToggleDir}
                        onOpenFile={onOpenFile}
                        rootPath=""
                    />
                </Paper>
            </Box>

            {/* Bottom: Costs summary */}
            <Box sx={{ mt: 1, mb: 2 }}>
                <Paper variant="outlined" sx={{ px: 1.5, py: 1, cursor: 'pointer' }} onClick={onOpenCosts}>
                    <Typography variant="subtitle2" fontWeight={600} mb={0.5}>
                        Costs so far
                    </Typography>
                    <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 0.25 }}>
                        <Typography variant="caption" color="text.secondary">Total</Typography>
                        <Typography variant="caption" fontWeight={600}>${((totalInUSD + totalOutUSD) || 0).toFixed(2)}</Typography>
                    </Box>
                    <Box sx={{ display: 'flex', justifyContent: 'space-between' }}>
                        <Typography variant="caption" color="text.secondary">In</Typography>
                        <Typography variant="caption" fontWeight={600}>${(totalInUSD || 0).toFixed(2)}</Typography>
                    </Box>
                    <Box sx={{ display: 'flex', justifyContent: 'space-between' }}>
                        <Typography variant="caption" color="text.secondary">Out</Typography>
                        <Typography variant="caption" fontWeight={600}>${(totalOutUSD || 0).toFixed(2)}</Typography>
                    </Box>
                </Paper>
            </Box>
        </Box>
    );
}

export default React.memo(Sidebar);


