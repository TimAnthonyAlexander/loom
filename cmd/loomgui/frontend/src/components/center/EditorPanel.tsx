import { Box, Tabs, Tab, IconButton } from '@mui/material';
import CloseIcon from '@mui/icons-material/Close';
import { EditorTabItem } from '../../types/ui';
import Editor, { OnMount } from '@monaco-editor/react';
import { guessLanguage } from '../../utils/language';

type Props = {
    openTabs: EditorTabItem[];
    activeTab: string;
    onChangeActiveTab: (path: string) => void;
    onCloseTab: (path: string) => void;
    onUpdateTab: (path: string, patch: Partial<EditorTabItem>) => void;
    onSaveTab: (path: string) => Promise<void>;
};

export default function EditorPanel({ openTabs, activeTab, onChangeActiveTab, onCloseTab, onUpdateTab, onSaveTab }: Props) {
    const tab = openTabs.find((t) => t.path === activeTab);

    const handleMount: OnMount = (editor, monaco) => {
        import('../../themes/mocha_converted.json').then((data: any) => {
            monaco.editor.defineTheme('catppuccin-mocha', data);
            monaco.editor.setTheme('catppuccin-mocha');
        });

        editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS, () => {
            if (tab?.path) onSaveTab(tab.path);
        });
        if (tab?.cursor) editor.setPosition({ lineNumber: tab.cursor.line, column: tab.cursor.column });
        setTimeout(() => editor.focus(), 0);
    };

    const handleChange = (value?: string) => {
        if (!tab) return;
        onUpdateTab(tab.path, { content: value ?? '', isDirty: true });
    };

    const language = tab?.language || (tab ? guessLanguage(tab.path) : 'text');

    return (
        <Box sx={{ display: 'flex', flexDirection: 'column', minWidth: 0, height: '100vh', borderRight: 1, borderColor: 'divider' }}>
            <Box sx={{ px: 2, pt: 1, borderBottom: 1, borderColor: 'divider' }}>
                <Tabs value={activeTab} onChange={(_, v) => onChangeActiveTab(v)} variant="scrollable" scrollButtons={false} sx={{ minHeight: 36 }}>
                    {openTabs.map((t) => (
                        <Tab
                            key={t.path}
                            label={
                                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                                    <span style={{ maxWidth: 180, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                                        {t.title}{t.isDirty ? ' •' : ''}
                                    </span>
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
            <Box sx={{ flex: 1, minHeight: 0 }}>
                {tab ? (
                    <Editor
                        height="100%"
                        defaultLanguage={language}
                        language={language}
                        value={tab.content}
                        onChange={handleChange}
                        onMount={handleMount}
                        options={{
                            fontSize: 13,
                            minimap: { enabled: false },
                            wordWrap: 'off',
                            lineNumbers: 'on',
                            automaticLayout: true,
                            renderWhitespace: 'selection',
                            tabSize: 4,
                            insertSpaces: true,
                            smoothScrolling: true,
                            scrollBeyondLastLine: false,
                        }}
                    />
                ) : (
                    <Box sx={{ p: 4, color: 'text.secondary' }}>Open a file from the left to view it here.</Box>
                )}
            </Box>
        </Box>
    );
}


