import { Box, Paper, Tabs, Tab, IconButton } from '@mui/material';
import CloseIcon from '@mui/icons-material/Close';
import { EditorTabItem } from '../../types/ui';

type Props = {
    openTabs: EditorTabItem[];
    activeTab: string;
    onChangeActiveTab: (path: string) => void;
    onCloseTab: (path: string) => void;
};

export default function EditorPanel({ openTabs, activeTab, onChangeActiveTab, onCloseTab }: Props) {
    return (
        <Box sx={{ display: 'flex', flexDirection: 'column', minWidth: 0, height: '100vh', borderRight: 1, borderColor: 'divider' }}>
            <Box sx={{ px: 2, pt: 1, borderBottom: 1, borderColor: 'divider' }}>
                <Tabs value={activeTab} onChange={(_, v) => onChangeActiveTab(v)} variant="scrollable" scrollButtons={false} sx={{ minHeight: 36 }}>
                    {openTabs.map((t) => (
                        <Tab
                            key={t.path}
                            label={
                                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                                    <span style={{ maxWidth: 180, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{t.title}</span>
                                    <IconButton size="small" onClick={(e) => { e.stopPropagation(); onCloseTab(t.path); }}>
                                        <CloseIcon fontSize="small" />
                                    </IconButton>
                                </Box>
                            }
                            value={t.path}
                            sx={{ minHeight: 32, fontSize: 12 }}
                        />
                    ))}
                </Tabs>
            </Box>
            <Box sx={{ flex: 1, overflow: 'auto' }}>
                {activeTab ? (
                    <Box sx={{ p: 2 }}>
                        <Paper variant="outlined" sx={{ p: 1.5, fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace', fontSize: 12, lineHeight: 1.5, whiteSpace: 'pre', overflowX: 'auto' }}>
                            {openTabs.find((t) => t.path === activeTab)?.content || ''}
                        </Paper>
                    </Box>
                ) : (
                    <Box sx={{ p: 4, color: 'text.secondary' }}>Open a file from the left to view it here.</Box>
                )}
            </Box>
        </Box>
    );
}


