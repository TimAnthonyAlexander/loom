import React from 'react';
import { Box, Typography, IconButton, Tooltip, Paper } from '@mui/material';
import SettingsIcon from '@mui/icons-material/Settings';
import RuleIcon from '@mui/icons-material/Rule';
import FileExplorer from './Files/FileExplorer';
import { UIFileEntry } from '../../types/ui';

type SidebarProps = {
    onOpenWorkspace: () => void;
    onOpenRules: () => void;
    onOpenSettings: () => void;
    onOpenCosts: () => void;
    totalInUSD: number;
    totalOutUSD: number;
    dirCache: Record<string, UIFileEntry[]>;
    expandedDirs: Record<string, boolean>;
    onToggleDir: (path: string) => void;
    onOpenFile: (path: string) => void;
};

function Sidebar(props: SidebarProps) {
    const {
        onOpenWorkspace,
        onOpenRules,
        onOpenSettings,
        onOpenCosts,
        totalInUSD,
        totalOutUSD,
        dirCache,
        expandedDirs,
        onToggleDir,
        onOpenFile,
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
                    <Tooltip title="Settings">
                        <IconButton size="small" onClick={onOpenSettings}>
                            <SettingsIcon fontSize="small" />
                        </IconButton>
                    </Tooltip>
                </Box>
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
                    <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 0.25 }}>
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


