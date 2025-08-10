import React from 'react';
import { Box, Button, Divider } from '@mui/material';
import MessageList from './MessageList';
import Composer from './Composer';
import { ChatMessage, ConversationListItem } from '../../../types/ui';
import ConversationList from '@/components/left/Conversations/ConversationList';

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
    } = props;

    return (
        <Box sx={{ width: 420, display: 'flex', flexDirection: 'column', minWidth: 0, height: '100vh' }}>
            <Box
                sx={{
                    width: '100%',
                    display: 'flex',
                    flexDirection: 'column',
                    alignItems: 'center',
                    justifyContent: 'center',
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
                <Composer input={input} setInput={setInput} busy={busy} onSend={onSend} onClear={onClear} />
            </Box>
        </Box>
    );
}


