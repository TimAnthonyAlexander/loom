import { Box, Tabs, Tab, IconButton } from '@mui/material';
import CloseIcon from '@mui/icons-material/Close';
import { EditorTabItem } from '../../types/ui';
import Editor, { OnMount } from '@monaco-editor/react';
import React from 'react';
import { guessLanguage } from '../../utils/language';

type Props = {
    openTabs: EditorTabItem[];
    activeTab: string;
    onChangeActiveTab: (path: string) => void;
    onCloseTab: (path: string) => void;
    onUpdateTab: (path: string, patch: Partial<EditorTabItem>) => void;
    onSaveTab: (path: string) => Promise<void>;
};

function EditorPanel({ openTabs, activeTab, onChangeActiveTab, onCloseTab, onUpdateTab, onSaveTab }: Props) {
    const tab = openTabs.find((t) => t.path === activeTab);
    const editorRef = React.useRef<any>(null);
    const monacoRef = React.useRef<any>(null);

    const handleMount: OnMount = (editor, monaco) => {
        import('../../themes/teal_converted.json').then((data: any) => {
            monaco.editor.defineTheme('catppuccin-mocha', data);
            monaco.editor.setTheme('catppuccin-mocha');
        });

        editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS, () => {
            if (tab?.path) onSaveTab(tab.path);
        });
        // Override Cmd/Ctrl+I inside Monaco to focus the chat composer instead of triggering suggestions
        try {
            editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyI, () => {
                try { window.dispatchEvent(new CustomEvent('loom:focus-composer')); } catch { }
            });
        } catch { }
        editorRef.current = editor;
        monacoRef.current = monaco;
        if (tab?.cursor) editor.setPosition({ lineNumber: tab.cursor.line, column: tab.cursor.column });
        setTimeout(() => editor.focus(), 0);
    };

    React.useEffect(() => {
        const editor = editorRef.current;
        if (!editor || !tab?.cursor) return;
        editor.setPosition({ lineNumber: tab.cursor.line, column: tab.cursor.column });
        editor.revealPositionInCenter({ lineNumber: tab.cursor.line, column: tab.cursor.column });
    }, [tab?.cursor?.line, tab?.cursor?.column, tab?.path]);

    const handleChange = (value?: string) => {
        if (!tab) return;
        onUpdateTab(tab.path, { content: value ?? '', isDirty: true });
    };

    const language = tab?.language || (tab ? guessLanguage(tab.path) : 'text');

    return (
        <Box sx={{ display: 'flex', flexDirection: 'column', minWidth: 0, maxWidth: '100%', height: '100vh', borderRight: 1, borderColor: 'divider' }}>
            <Box sx={{ px: 2, pt: 1, borderBottom: 1, borderColor: 'divider', minWidth: 0, maxWidth: '100%', overflow: 'hidden' }}>
                <Tabs
                    value={activeTab}
                    onChange={(_, v) => onChangeActiveTab(v)}
                    variant="scrollable"
                    scrollButtons="auto"
                    sx={{
                        minHeight: 36,
                        minWidth: 0,
                        maxWidth: '100%',
                        '& .MuiTabs-scroller': {
                            overflow: 'hidden !important'
                        },
                        '& .MuiTabs-flexContainer': {
                            minWidth: 0
                        }
                    }}
                >
                    {openTabs.map((t) => (
                        <Tab
                            key={t.path}
                            label={
                                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, minWidth: 0, maxWidth: 200 }}>
                                    <span style={{ maxWidth: 140, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', minWidth: 0 }}>
                                        {t.title}{t.isDirty ? ' •' : ''}
                                    </span>
                                    <IconButton size="small" onClick={(e) => { e.stopPropagation(); onCloseTab(t.path); }}>
                                        <CloseIcon fontSize="small" />
                                    </IconButton>
                                </Box>
                            }
                            value={t.path}
                            sx={{ minHeight: 32, fontSize: 12, minWidth: 0, maxWidth: 200 }}
                        />
                    ))}
                </Tabs>
            </Box>

            <Box sx={{ flex: 1, minHeight: 0, minWidth: 0, maxWidth: '100%', overflow: 'hidden', width: '100%' }}>
                {tab ? (
                    <div style={{ width: '100%', height: '100%', maxWidth: '100%', overflow: 'hidden' }}>
                        <Editor
                            width="100%"
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
                                scrollbar: {
                                    horizontal: 'auto',
                                    vertical: 'auto'
                                }
                            }}
                        />
                    </div>
                ) : (
                    <Box sx={{ p: 4, color: 'text.secondary' }}></Box>
                )}
            </Box>
        </Box>
    );
}

export default React.memo(EditorPanel, (prev, next) => {
    return (
        prev.activeTab === next.activeTab &&
        prev.openTabs === next.openTabs &&
        prev.onChangeActiveTab === next.onChangeActiveTab &&
        prev.onCloseTab === next.onCloseTab &&
        prev.onUpdateTab === next.onUpdateTab &&
        prev.onSaveTab === next.onSaveTab
    );
});


