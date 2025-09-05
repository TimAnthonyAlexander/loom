import React from 'react';
import { Box, Typography, IconButton, Tooltip, LinearProgress } from '@mui/material';
import SettingsIcon from '@mui/icons-material/Settings';
import RuleIcon from '@mui/icons-material/Rule';
import MemoryIcon from '@mui/icons-material/BookmarkBorder';
import { EventsOn } from '../../../wailsjs/runtime/runtime';
import FileExplorer from './Files/FileExplorer';
import ProfileDialog from '../dialogs/ProfileDialog';
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

    const [symbolsCount, setSymbolsCount] = React.useState<number | null>(null);
    const [profileDialogOpen, setProfileDialogOpen] = React.useState(false);

    const fetchSymbolsCount = React.useCallback(async () => {
        try {
            const anyWin: any = window as any;
            const n = await anyWin?.go?.bridge?.App?.GetSymbolsCount?.();
            if (typeof n === 'number') {
                setSymbolsCount(n);
            }
        } catch {
            // ignore
        }
    }, []);

    React.useEffect(() => {
        fetchSymbolsCount();
    }, [fetchSymbolsCount]);

    React.useEffect(() => {
        if (indexing && indexing.status === 'done') {
            fetchSymbolsCount();
        }
    }, [indexing?.status, fetchSymbolsCount]);

    // Listen for workspace changes and refetch symbol count
    React.useEffect(() => {
        const handleWorkspaceChanged = () => {
            // Reset symbol count immediately and fetch new count
            setSymbolsCount(null);
            // Add a small delay to ensure the backend has time to initialize the new symbols service
            setTimeout(() => {
                fetchSymbolsCount();
            }, 500);
        };

        const off = EventsOn('workspace:changed', handleWorkspaceChanged);

        return () => {
            off();
        };
    }, [fetchSymbolsCount]);

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
            <Box
                sx={{
                    pt: 4,
                    borderBottom: '0.1px solid #ccc',
                }}
            >
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.5 }}>
                    <Box component="img" src="/logo.png" alt="Loom" sx={{ width: 28, height: 28, borderRadius: 0.5 }} />
                    <Typography
                        variant="h6"
                        fontWeight={700}
                        sx={{
                            background: 'linear-gradient(45deg, currentColor, primary.main)',
                            backgroundClip: 'text',
                            WebkitBackgroundClip: 'text',
                            color: 'text.primary',
                        }}
                    >
                        Loom
                    </Typography>
                    <Box sx={{ flex: 1 }} />
                    <Tooltip title="Select Workspace">
                        <IconButton
                            size="small"
                            onClick={onOpenWorkspace}
                            sx={{
                                color: 'text.secondary',
                                '&:hover': {
                                    color: 'primary.main',
                                    backgroundColor: 'primary.main',
                                    '& .MuiTypography-root': {
                                        color: 'primary.contrastText',
                                        fontWeight: 600
                                    }
                                }
                            }}
                        >
                            <Typography variant="caption">WS</Typography>
                        </IconButton>
                    </Tooltip>
                    <Tooltip title="Rules">
                        <IconButton
                            size="small"
                            onClick={onOpenRules}
                            sx={{
                                color: 'text.secondary',
                                '&:hover': {
                                    backgroundColor: 'primary.main',
                                    '& .MuiSvgIcon-root': {
                                        color: 'primary.contrastText'
                                    }
                                }
                            }}
                        >
                            <RuleIcon fontSize="small" />
                        </IconButton>
                    </Tooltip>
                    <Tooltip title="Memories">
                        <IconButton
                            size="small"
                            onClick={onOpenMemories}
                            sx={{
                                color: 'text.secondary',
                                '&:hover': {
                                    backgroundColor: 'primary.main',
                                    '& .MuiSvgIcon-root': {
                                        color: 'primary.contrastText'
                                    }
                                }
                            }}
                        >
                            <MemoryIcon fontSize="small" />
                        </IconButton>
                    </Tooltip>
                    <Tooltip title="Settings">
                        <IconButton
                            size="small"
                            onClick={onOpenSettings}
                            sx={{
                                color: 'text.secondary',
                                '&:hover': {
                                    backgroundColor: 'primary.main',
                                    '& .MuiSvgIcon-root': {
                                        color: 'primary.contrastText'
                                    }
                                }
                            }}
                        >
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
                <Tooltip title="Click to view project profile">
                    <Box
                        sx={{
                            mt: 1,
                            display: 'flex',
                            alignItems: 'center',
                            gap: 1,
                            cursor: 'pointer',
                            p: 0.5,
                            borderRadius: 1,
                            '&:hover': {
                                bgcolor: 'action.hover'
                            }
                        }}
                        onClick={() => setProfileDialogOpen(true)}
                    >
                        <Typography variant="caption" fontWeight={600}>
                            {symbolsCount !== null ? 'Indexed' : 'Indexing'}
                        </Typography>
                    </Box>
                </Tooltip>
            </Box>

            <Box
                sx={{
                    flex: 1,
                    minHeight: 0,
                    display: 'flex',
                    flexDirection: 'column',
                    overflowY: 'auto',
                }}
            >
                <FileExplorer
                    dirCache={dirCache}
                    expandedDirs={expandedDirs}
                    onToggleDir={onToggleDir}
                    onOpenFile={onOpenFile}
                    rootPath=""
                />
            </Box>

            {/* Bottom: Costs summary */}
            <Box
                sx={{
                    px: 1.5,
                    py: 1,
                    cursor: 'pointer',
                    transition: 'all 0.2s ease-in-out',
                    borderTop: '0.1px solid #ccc',
                }}
                onClick={onOpenCosts}
            >
                <Typography variant="subtitle2" fontWeight={600} mb={0.5}>
                    Costs so far
                </Typography>
                <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 0.25 }}>
                    <Typography variant="caption" color="text.secondary">Total</Typography>
                    <Typography
                        variant="caption"
                        fontWeight={600}
                        sx={{
                            color: 'primary.main',
                            '.MuiPaper-root:hover &': {
                                color: 'inherit'
                            }
                        }}
                    >
                        ${((totalInUSD + totalOutUSD) || 0).toFixed(2)}
                    </Typography>
                </Box>
                <Box sx={{ display: 'flex', justifyContent: 'space-between' }}>
                    <Typography variant="caption" color="text.secondary">In</Typography>
                    <Typography variant="caption" fontWeight={600}>${(totalInUSD || 0).toFixed(2)}</Typography>
                </Box>
                <Box sx={{ display: 'flex', justifyContent: 'space-between' }}>
                    <Typography variant="caption" color="text.secondary">Out</Typography>
                    <Typography variant="caption" fontWeight={600}>${(totalOutUSD || 0).toFixed(2)}</Typography>
                </Box>
            </Box>

            <ProfileDialog
                open={profileDialogOpen}
                onClose={() => setProfileDialogOpen(false)}
            />
        </Box>
    );
}

export default React.memo(Sidebar);


