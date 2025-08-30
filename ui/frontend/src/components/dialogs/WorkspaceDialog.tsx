import { 
    Dialog, 
    DialogTitle, 
    DialogContent, 
    DialogActions, 
    Button, 
    Stack, 
    TextField, 
    Typography,
    Divider,
    List,
    ListItem,
    ListItemButton,
    ListItemText,
    Box,
    Chip
} from '@mui/material';
import CreateNewFolderIcon from '@mui/icons-material/CreateNewFolder';
import HistoryIcon from '@mui/icons-material/History';
import FolderIcon from '@mui/icons-material/Folder';

type Props = {
    open: boolean;
    workspacePath: string;
    setWorkspacePath: (v: string) => void;
    onBrowse: () => void;
    onUse: () => void;
    onClose: () => void;
    onNewProject: () => void;
    recentWorkspaces: string[];
    onOpenRecent: (path: string) => void;
};

export default function WorkspaceDialog({ open, workspacePath, setWorkspacePath, onBrowse, onUse, onClose, onNewProject, recentWorkspaces, onOpenRecent }: Props) {
    const formatPath = (path: string) => {
        // Show only the last two parts of the path for better readability
        const parts = path.replace(/\\/g, '/').split('/').filter(p => p);
        if (parts.length <= 2) return path;
        return '.../' + parts.slice(-2).join('/');
    };

    return (
        <Dialog open={open} onClose={onClose} maxWidth="md" fullWidth>
            <DialogTitle sx={{ 
                background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
                color: 'white',
                borderRadius: '4px 4px 0 0'
            }}>
                Select Workspace
            </DialogTitle>
            <DialogContent dividers sx={{ pb: 3 }}>
                <Stack spacing={3} sx={{ mt: 2 }}>
                    {/* Quick Actions */}
                    <Box>
                        <Typography variant="subtitle2" sx={{ mb: 2, fontWeight: 600, color: 'text.primary' }}>
                            Quick Actions
                        </Typography>
                        <Stack direction="row" spacing={2}>
                            <Button
                                variant="outlined"
                                startIcon={<CreateNewFolderIcon />}
                                onClick={() => {
                                    onClose();
                                    onNewProject();
                                }}
                                sx={{
                                    borderColor: '#667eea',
                                    color: '#667eea',
                                    '&:hover': {
                                        borderColor: '#764ba2',
                                        color: '#764ba2',
                                        backgroundColor: 'rgba(102, 126, 234, 0.04)'
                                    }
                                }}
                            >
                                New Project
                            </Button>
                        </Stack>
                    </Box>

                    <Divider />

                    {/* Open Existing */}
                    <Box>
                        <Typography variant="subtitle2" sx={{ mb: 2, fontWeight: 600, color: 'text.primary' }}>
                            Open Existing Workspace
                        </Typography>
                        <Stack direction="row" spacing={1}>
                            <TextField 
                                label="Workspace Path" 
                                value={workspacePath} 
                                onChange={(e) => setWorkspacePath(e.target.value)} 
                                placeholder="/path/to/project" 
                                fullWidth 
                                variant="outlined"
                            />
                            <Button 
                                variant="outlined" 
                                onClick={onBrowse}
                                sx={{ 
                                    minWidth: 'auto',
                                    px: 2,
                                    borderColor: '#667eea',
                                    color: '#667eea',
                                    '&:hover': {
                                        borderColor: '#764ba2',
                                        color: '#764ba2',
                                        backgroundColor: 'rgba(102, 126, 234, 0.04)'
                                    }
                                }}
                            >
                                <FolderIcon />
                            </Button>
                        </Stack>
                        <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
                            Enter a project directory. Project rules will be stored in <code>.loom/rules.json</code> under this path.
                        </Typography>
                    </Box>

                    {/* Recent Workspaces */}
                    {recentWorkspaces.length > 0 && (
                        <>
                            <Divider />
                            <Box>
                                <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 2 }}>
                                    <HistoryIcon sx={{ color: '#667eea', fontSize: 20 }} />
                                    <Typography variant="subtitle2" sx={{ fontWeight: 600, color: 'text.primary' }}>
                                        Recent Workspaces
                                    </Typography>
                                    <Chip 
                                        label={recentWorkspaces.length} 
                                        size="small" 
                                        sx={{ 
                                            backgroundColor: '#667eea',
                                            color: 'white',
                                            fontWeight: 600,
                                            minWidth: '24px',
                                            height: '20px'
                                        }} 
                                    />
                                </Stack>
                                <List dense sx={{ maxHeight: 200, overflow: 'auto' }}>
                                    {recentWorkspaces.slice(0, 8).map((path, index) => (
                                        <ListItem key={index} disablePadding>
                                            <ListItemButton
                                                onClick={() => {
                                                    onOpenRecent(path);
                                                }}
                                                sx={{
                                                    borderRadius: 1,
                                                    mb: 0.5,
                                                    '&:hover': {
                                                        backgroundColor: 'rgba(102, 126, 234, 0.04)',
                                                        '& .MuiListItemText-primary': {
                                                            color: '#667eea'
                                                        }
                                                    }
                                                }}
                                            >
                                                <ListItemText
                                                    primary={formatPath(path)}
                                                    secondary={path}
                                                    primaryTypographyProps={{
                                                        fontWeight: 500,
                                                        fontSize: '0.875rem'
                                                    }}
                                                    secondaryTypographyProps={{
                                                        fontSize: '0.75rem',
                                                        sx: { 
                                                            fontFamily: 'monospace',
                                                            opacity: 0.7,
                                                            whiteSpace: 'nowrap',
                                                            overflow: 'hidden',
                                                            textOverflow: 'ellipsis',
                                                            maxWidth: '100%'
                                                        }
                                                    }}
                                                />
                                            </ListItemButton>
                                        </ListItem>
                                    ))}
                                </List>
                                {recentWorkspaces.length > 8 && (
                                    <Typography variant="caption" color="text.secondary" sx={{ mt: 1, display: 'block', textAlign: 'center' }}>
                                        and {recentWorkspaces.length - 8} more...
                                    </Typography>
                                )}
                            </Box>
                        </>
                    )}
                </Stack>
            </DialogContent>
            <DialogActions sx={{ p: 2, gap: 1 }}>
                <Button onClick={onClose} color="inherit">
                    Cancel
                </Button>
                <Button 
                    variant="contained" 
                    onClick={onUse} 
                    disabled={!workspacePath.trim()}
                    sx={{
                        background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
                        '&:hover': {
                            background: 'linear-gradient(135deg, #5a6fd8 0%, #6a4190 100%)',
                        },
                        '&:disabled': {
                            background: 'grey.300',
                        }
                    }}
                >
                    Open Workspace
                </Button>
            </DialogActions>
        </Dialog>
    );
}


