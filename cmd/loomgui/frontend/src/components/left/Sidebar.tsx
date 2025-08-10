import { Box, Typography, IconButton, Tooltip, Paper } from '@mui/material';
import SettingsIcon from '@mui/icons-material/Settings';
import RuleIcon from '@mui/icons-material/Rule';
import ModelSelector from '../../ModelSelector';
import FileExplorer from './Files/FileExplorer';
import { UIFileEntry } from '../../types/ui';

type SidebarProps = {
    onOpenWorkspace: () => void;
    onOpenRules: () => void;
    onOpenSettings: () => void;
    dirCache: Record<string, UIFileEntry[]>;
    expandedDirs: Record<string, boolean>;
    onToggleDir: (path: string) => void;
    onOpenFile: (path: string) => void;
};

export default function Sidebar(props: SidebarProps) {
    const {
        onOpenWorkspace,
        onOpenRules,
        onOpenSettings,
        dirCache,
        expandedDirs,
        onToggleDir,
        onOpenFile,
    } = props;

    return (
        <Box sx={{ px: 1, py: 1, display: 'flex', flexDirection: 'column', gap: 2, overflowY: 'auto' }}>
            <Box sx={{ pt: 4 }}>
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.5 }}>
                    <Box component="img" src="/logo.png" alt="Loom" sx={{ width: 28, height: 28, borderRadius: 0.5 }} />
                    <Typography variant="h6" fontWeight={600}>
                        Loom v2
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

            <Box>
                <Paper variant="outlined" sx={{ overflowY: 'auto', height: '100%', }}>
                    <FileExplorer
                        dirCache={dirCache}
                        expandedDirs={expandedDirs}
                        onToggleDir={onToggleDir}
                        onOpenFile={onOpenFile}
                        rootPath=""
                    />
                </Paper>
            </Box>
        </Box>
    );
}


