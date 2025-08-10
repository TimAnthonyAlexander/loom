import React from 'react';
import { Box, Button, Divider } from '@mui/material';
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
    input: string;
    setInput: (val: string) => void;
    onSend: () => void;
    onClear: () => void;
    messagesEndRef: React.RefObject<HTMLDivElement>;
    onNewConversation: () => void;
    conversations: ConversationListItem[];
    currentConversationId: string;
    onSelectConversation: (id: string) => void;
    currentModel: string;
    onSelectModel: (model: string) => void;
};

export default function ChatPanel(props: Props) {
    const {
        messages,
        busy,
        lastUserIdx,
        reasoningText,
        reasoningOpen,
        onToggleReasoning,
        input,
        setInput,
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

    React.useEffect(() => {
        const onKeyDown = (e: KeyboardEvent) => {
            const isMac = navigator.platform.toLowerCase().includes('mac');
            const cmd = isMac ? e.metaKey : e.ctrlKey;
            const key = e.key.toLowerCase();
            if (cmd && key === 'i') {
                e.preventDefault();
                focusTokenRef.current += 1;
                // Trigger a rerender by updating a state-likes value
                setFocusBump(focusTokenRef.current);
            }
        };
        window.addEventListener('keydown', onKeyDown);
        return () => window.removeEventListener('keydown', onKeyDown);
    }, []);

    const [focusBump, setFocusBump] = React.useState<number>(0);

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
                    <ConversationList
                        conversations={conversations}
                        currentConversationId={currentConversationId}
                        onSelect={onSelectConversation}
                    />
                }
            </Box>
            <Divider />
            <Box sx={{ px: 3, py: 2 }}>
                <Composer input={input} setInput={setInput} busy={busy} onSend={onSend} onClear={onClear} focusToken={focusBump} />
            </Box>
        </Box>
    );
}


