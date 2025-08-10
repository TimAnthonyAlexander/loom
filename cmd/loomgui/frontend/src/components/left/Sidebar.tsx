import { Box, Typography, IconButton, Tooltip, Paper, Button } from '@mui/material';
import SettingsIcon from '@mui/icons-material/Settings';
import RuleIcon from '@mui/icons-material/Rule';
import ModelSelector from '../../ModelSelector';
import FileExplorer from './Files/FileExplorer';
import { UIFileEntry, ConversationListItem } from '../../types/ui';

type SidebarProps = {
    onOpenWorkspace: () => void;
    onOpenRules: () => void;
    onOpenSettings: () => void;
    currentModel: string;
    onSelectModel: (model: string) => void;
    dirCache: Record<string, UIFileEntry[]>;
    expandedDirs: Record<string, boolean>;
    onToggleDir: (path: string) => void;
    onOpenFile: (path: string) => void;
    conversations: ConversationListItem[];
    currentConversationId: string;
    onSelectConversation: (id: string) => void;
    onNewConversation: () => void;
};

export default function Sidebar(props: SidebarProps) {
    const {
        onOpenWorkspace,
        onOpenRules,
        onOpenSettings,
        currentModel,
        onSelectModel,
        dirCache,
        expandedDirs,
        onToggleDir,
        onOpenFile,
        onNewConversation,
    } = props;

    return (
        <Box sx={{ px: 2, py: 2, display: 'flex', flexDirection: 'column', gap: 2, overflowY: 'auto' }}>
            <Box sx={{ pt: 2 }}>
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
                <Typography variant="body2" color="text.secondary">
                    Minimal, calm, precise.
                </Typography>
            </Box>

            <Box>
                <ModelSelector onSelect={onSelectModel} currentModel={currentModel} />
            </Box>

            <Box>
                <Typography variant="subtitle2" sx={{ mb: 1 }}>Files</Typography>
                <Paper variant="outlined" sx={{ p: 1, maxHeight: '60vh', overflowY: 'auto' }}>
                    <FileExplorer
                        dirCache={dirCache}
                        expandedDirs={expandedDirs}
                        onToggleDir={onToggleDir}
                        onOpenFile={onOpenFile}
                        rootPath=""
                    />
                </Paper>
            </Box>

            <Box>
                <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 1 }}>
                    <Button size="small" variant="outlined" onClick={onNewConversation}>New Conversation</Button>
                </Box>
            </Box>
        </Box>
    );
}


