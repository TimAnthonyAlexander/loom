import { Box, List, ListItem, ListItemText, Typography } from '@mui/material';
import { UIFileEntry } from '../../../types/ui';

type Props = {
    dirCache: Record<string, UIFileEntry[]>;
    expandedDirs: Record<string, boolean>;
    onToggleDir: (path: string) => void;
    onOpenFile: (path: string) => void;
    rootPath?: string;
};

export default function FileExplorer({ dirCache, expandedDirs, onToggleDir, onOpenFile, rootPath = '' }: Props) {
    const renderDir = (path: string) => {
        const key = path || '';
        const entries = dirCache[key] || [];
        return (
            <List dense disablePadding>
                {entries.map((e) => {
                    const childKey = e.path;
                    if (e.is_dir) {
                        const isOpen = !!expandedDirs[childKey];
                        return (
                            <Box key={childKey} sx={{ pl: path ? 1.5 : 0.5 }}>
                                <ListItem disableGutters sx={{ cursor: 'pointer' }} onClick={() => onToggleDir(childKey)}>
                                    <ListItemText
                                        primaryTypographyProps={{ fontFamily: 'ui-monospace, Menlo, monospace', fontSize: 13 }}
                                        primary={`${isOpen ? '▼' : '▶'} ${e.name}`}
                                    />
                                </ListItem>
                                {isOpen && (dirCache[childKey] ? (
                                    <Box sx={{ pl: 1 }}>{renderDir(childKey)}</Box>
                                ) : (
                                    <Box sx={{ pl: 2, py: 0.5, color: 'text.secondary' }}>
                                        <Typography variant="caption">Loading…</Typography>
                                    </Box>
                                ))}
                            </Box>
                        );
                    }
                    return (
                        <ListItem
                            key={childKey}
                            disableGutters
                            sx={{ pl: path ? 3.5 : 2, cursor: 'pointer' }}
                            onClick={() => onOpenFile(childKey)}
                        >
                            <ListItemText
                                primaryTypographyProps={{ fontFamily: 'ui-monospace, Menlo, monospace', fontSize: 13 }}
                                primary={e.name}
                            />
                        </ListItem>
                    );
                })}
            </List>
        );
    };

    return <>{renderDir(rootPath)}</>;
}


