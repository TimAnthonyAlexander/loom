import React from 'react';
import { Box, Button, Divider, Typography, Popover, TextField, List, ListItemButton, ListItemText } from '@mui/material';
import * as AppBridge from '../../../../wailsjs/go/bridge/App';
import * as Bridge from '../../../../wailsjs/go/bridge/App';
import MessageList from './MessageList';
import Composer from './Composer';
import { ChatMessage, ConversationListItem } from '../../../types/ui';
import ConversationList from '@/components/left/Conversations/ConversationList';
import ModelSelector from '@/ModelSelector';

type Props = {
    messages: ChatMessage[];
    busy: boolean;
    lastUserIdx: number;
    reasoningText: string;
    reasoningOpen: boolean;
    onToggleReasoning: (open: boolean) => void;
    onSend: (text: string) => void;
    onClear: () => void;
    messagesEndRef: React.RefObject<HTMLDivElement>;
    onNewConversation: () => void;
    conversations: ConversationListItem[];
    currentConversationId: string;
    onSelectConversation: (id: string) => void;
    currentModel: string;
    onSelectModel: (model: string) => void;
};

function ChatPanelComponent(props: Props) {
    const {
        messages,
        busy,
        lastUserIdx,
        reasoningText,
        reasoningOpen,
        onToggleReasoning,
        onSend,
        onClear,
        messagesEndRef,
        onNewConversation,
        conversations,
        currentConversationId,
        onSelectConversation,
        currentModel,
        onSelectModel,
    } = props;

    const focusTokenRef = React.useRef<number>(0);
    const [localInput, setLocalInput] = React.useState<string>('');

    React.useEffect(() => {
        const onKeyDown = (e: KeyboardEvent) => {
            const isMac = navigator.platform.toLowerCase().includes('mac');
            const cmd = isMac ? e.metaKey : e.ctrlKey;
            const key = e.key.toLowerCase();
            if (cmd && key === 'i') {
                e.preventDefault();
                focusTokenRef.current += 1;
                setFocusBump(focusTokenRef.current);
            }
        };
        const onFocusComposer = () => {
            focusTokenRef.current += 1;
            setFocusBump(focusTokenRef.current);
        };
        window.addEventListener('keydown', onKeyDown);
        window.addEventListener('loom:focus-composer', onFocusComposer as EventListener);
        return () => {
            window.removeEventListener('keydown', onKeyDown);
            window.removeEventListener('loom:focus-composer', onFocusComposer as EventListener);
        };
    }, []);

    const [focusBump, setFocusBump] = React.useState<number>(0);

    const [attachments, setAttachments] = React.useState<string[]>([]);
    const [attachOpen, setAttachOpen] = React.useState<boolean>(false);
    const [attachQuery, setAttachQuery] = React.useState<string>('');
    const [attachResults, setAttachResults] = React.useState<string[]>([]);
    const [attachIndex, setAttachIndex] = React.useState<number>(0);
    const [attachAnchor, setAttachAnchor] = React.useState<HTMLElement | null>(null);

    // Clear attachments when backend emits chat:clear
    React.useEffect(() => {
        const handler = () => setAttachments([]);
        const fn = () => handler();
        (window as any).wails?.EventsOn?.('chat:clear', fn);
        // Also listen to our local event bus for safety
        window.addEventListener('loom:clear-attachments', handler as EventListener);
        return () => {
            window.removeEventListener('loom:clear-attachments', handler as EventListener);
        };
    }, []);

    // Open attach popup via global shortcut broadcast from App
    React.useEffect(() => {
        const handler = () => setAttachOpen(true);
        window.addEventListener('loom:open-attach', handler as EventListener);
        return () => window.removeEventListener('loom:open-attach', handler as EventListener);
    }, []);

    // Debounce helper for attachQuery
    const useDebounced = (value: string, delayMs: number) => {
        const [debounced, setDebounced] = React.useState(value);
        React.useEffect(() => {
            const t = setTimeout(() => setDebounced(value), delayMs);
            return () => clearTimeout(t);
        }, [value, delayMs]);
        return debounced;
    };
    const debouncedQuery = useDebounced(attachQuery, 150);

    // Query filenames using backend FindFiles
    React.useEffect(() => {
        if (!attachOpen) return;
                        if (!debouncedQuery.trim()) { setAttachResults([]); setAttachIndex(0); return; }
        (AppBridge as any).FindFiles(debouncedQuery, '', 200)
            .then((list: any) => {
                const arr = Array.isArray(list) ? (list as string[]) : [];
                                // Deduplicate and filter out directories (best-effort; FindFiles returns files)
                                const seen: Record<string, boolean> = {};
                                const dedup = arr.filter((p) => {
                                    const k = String(p).toLowerCase();
                                    if (seen[k]) return false; seen[k] = true; return true;
                                });
                                setAttachResults(dedup);
                setAttachIndex(0);
            })
            .catch(() => setAttachResults([]));
    }, [attachOpen, debouncedQuery]);

    // Push attachments to backend so the engine can inject previews
    React.useEffect(() => {
        try {
            if ((AppBridge as any).SetAttachments) {
                (AppBridge as any).SetAttachments(attachments);
            } else if ((window as any).wails?.EventsEmit) {
                (window as any).wails.EventsEmit('chat:set_attachments', attachments);
            }
        } catch { }
    }, [attachments]);

    return (
        <Box sx={{ minWidth: 420, width: '20%', display: 'flex', flexDirection: 'column', height: '100vh' }}>
            <Box
                sx={{
                    width: '100%',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    p: 2,
                    borderBottom: 1,
                    borderColor: 'divider',
                }}
            >
                <Button
                    size="small"
                    variant="outlined"
                    onClick={onNewConversation}
                >
                    New Conversation
                </Button>
                <ModelSelector onSelect={onSelectModel} currentModel={currentModel} />
            </Box>
            <Box sx={{ flex: 1, overflowY: 'auto', px: 3, py: 2, minHeight: 0 }}>
                <MessageList
                    messages={messages}
                    busy={busy}
                    lastUserIdx={lastUserIdx}
                    reasoningText={reasoningText}
                    reasoningOpen={reasoningOpen}
                    onToggleReasoning={onToggleReasoning}
                    messagesEndRef={messagesEndRef}
                />
                {messages.length === 0 &&
                    <Box>
                        <Typography
                            variant="subtitle2"
                            fontWeight={600}
                            sx={{
                                p: 1,
                            }}
                        >
                            Past Conversations
                        </Typography>
                        <ConversationList
                            conversations={conversations}
                            currentConversationId={currentConversationId}
                            onSelect={onSelectConversation}
                        />
                    </Box>
                }
            </Box>
            <Divider />
            <Box sx={{ px: 3, py: 2 }}>
                <Composer
                    input={localInput}
                    setInput={setLocalInput}
                    busy={busy}
                    onSend={async () => {
                        const text = localInput;
                        if (!text.trim() || busy) return;
                        let augmented = text;
                        if (attachments.length > 0) {
                            // Fetch first 50 lines for each attachment
                            const previews = await Promise.all(
                                attachments.map(async (p) => {
                                    try {
                                        const res: any = await Bridge.ReadWorkspaceFile(p);
                                        const content = String(res?.content || '');
                                        const lines = content.split('\n').slice(0, 50).join('\n');
                                        const name = p.split('/').pop() || p;
                                        return `- ${name} — ${p}\n  The user attached this file for additional context. Use it if relevant.\n  First 50 lines:\n` + lines.split('\n').map((l: string) => `    ${l}`).join('\n');
                                    } catch {
                                        const name = p.split('/').pop() || p;
                                        return `- ${name} — ${p}\n  (unreadable)`;
                                    }
                                })
                            );
                            const block = `<attachments>\nAttachments:\n${previews.join('\n')}\n</attachments>`;
                            augmented = `${text}\n\n${block}`;
                        }
                        onSend(augmented);
                        setLocalInput('');
                    }}
                    onClear={() => {
                        setLocalInput('');
                        onClear();
                    }}
                    focusToken={focusBump}
                    attachments={attachments}
                    onRemoveAttachment={(p) => setAttachments((prev) => prev.filter((x) => x !== p))}
                    onOpenAttach={(el) => { setAttachAnchor(el); setAttachOpen(true); }}
                    onAttachButtonRef={(el) => setAttachAnchor(el)}
                />
            </Box>
            <Popover
                open={attachOpen}
                anchorEl={attachAnchor}
                onClose={() => setAttachOpen(false)}
                anchorOrigin={{ vertical: 'top', horizontal: 'left' }}
                transformOrigin={{ vertical: 'bottom', horizontal: 'left' }}
                PaperProps={{ sx: { p: 1, width: 420 } }}
            >
                <TextField
                    autoFocus
                    fullWidth
                    placeholder="Type to fuzzy find…"
                    value={attachQuery}
                    onChange={(e) => setAttachQuery(e.target.value)}
                    onKeyDown={(e) => {
                        if (e.key === 'Enter') {
                            const item = attachResults[attachIndex];
                            if (item) {
                                setAttachments((prev) => (prev.includes(item) ? prev : [...prev, item]));
                                setAttachOpen(false);
                                setAttachQuery('');
                            }
                        } else if (e.key === 'ArrowDown') {
                            e.preventDefault();
                            setAttachIndex((i) => Math.min(Math.max(attachResults.length - 1, 0), i + 1));
                        } else if (e.key === 'ArrowUp') {
                            e.preventDefault();
                            setAttachIndex((i) => Math.max(0, i - 1));
                        }
                    }}
                    size="small"
                    sx={{ mb: 1 }}
                />
                <List dense sx={{ maxHeight: 320, overflowY: 'auto' }}>
                    {attachResults.map((p, idx) => (
                        <ListItemButton
                            key={p}
                            selected={idx === attachIndex}
                            onMouseEnter={() => setAttachIndex(idx)}
                            onClick={() => {
                                setAttachments((prev) => (prev.includes(p) ? prev : [...prev, p]));
                                setAttachOpen(false);
                                setAttachQuery('');
                            }}
                        >
                            <ListItemText primaryTypographyProps={{ fontFamily: 'ui-monospace, Menlo, monospace', fontSize: 13 }} primary={p} />
                        </ListItemButton>
                    ))}
                </List>
            </Popover>
        </Box>
    );
}

export default React.memo(ChatPanelComponent, (prev, next) => {
    return (
        prev.messages === next.messages &&
        prev.busy === next.busy &&
        prev.lastUserIdx === next.lastUserIdx &&
        prev.reasoningText === next.reasoningText &&
        prev.reasoningOpen === next.reasoningOpen &&
        prev.onSend === next.onSend &&
        prev.onClear === next.onClear &&
        prev.messagesEndRef === next.messagesEndRef &&
        prev.onNewConversation === next.onNewConversation &&
        prev.conversations === next.conversations &&
        prev.currentConversationId === next.currentConversationId &&
        prev.onSelectConversation === next.onSelectConversation &&
        prev.currentModel === next.currentModel &&
        prev.onSelectModel === next.onSelectModel
    );
});


