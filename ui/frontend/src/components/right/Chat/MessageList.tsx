import React from 'react';
import { Box, Stack, CircularProgress } from '@mui/material';
import MarkdownRenderer from '../../markdown/MarkdownRenderer';
import MarkdownErrorBoundary from '../../markdown/MarkdownErrorBoundary';
import ReasoningPanel from './ReasoningPanel';
import { ChatMessage } from '../../../types/ui';
// Remove attachment blocks from display while keeping them in message payloads
function filterAttachments(text: string): string {
    if (!text) return text;
    try {
        return text.replace(/<attachments>[\s\S]*?<\/attachments>/g, '').trim();
    } catch {
        return text;
    }
}

type Props = {
    messages: ChatMessage[];
    busy: boolean;
    lastUserIdx: number;
    reasoningText: string;
    reasoningOpen: boolean;
    onToggleReasoning: (open: boolean) => void;
    messagesEndRef: React.RefObject<HTMLDivElement>;
};

type ItemProps = {
    msg: ChatMessage;
    index: number;
    messagesCount: number;
    busy: boolean;
    showReasoning: boolean;
    reasoningText: string;
    reasoningOpen: boolean;
    onToggleReasoning: (open: boolean) => void;
};

const MessageItem = React.memo(function MessageItem({
    msg,
    index,
    messagesCount,
    busy,
    showReasoning,
    reasoningText,
    reasoningOpen,
    onToggleReasoning,
}: ItemProps) {
    const isUser = msg.role === 'user';
    const isSystem = msg.role === 'system';
    const isLastMessage = index === messagesCount - 1;
    const showSpinner = isSystem && isLastMessage && busy;

    if (isSystem) {
        return (
            <Box
                sx={{
                    borderRadius: 1.5,
                    color: 'text.secondary',
                }}
            >
                <Stack direction="row" spacing={1.25} alignItems="flex-start">
                    <Box sx={{ pt: '3px' }}>
                        {showSpinner &&
                            <CircularProgress size={14} thickness={5} />
                        }
                    </Box>
                    <Box sx={{ flex: 1, fontSize: '0.9rem' }}>
                        <MarkdownErrorBoundary>
                            <MarkdownRenderer>{msg.content}</MarkdownRenderer>
                        </MarkdownErrorBoundary>
                    </Box>
                </Stack>
            </Box>
        );
    }

    const containerProps = isUser
        ? { component: Box, sx: { p: 2, border: '1px solid', borderColor: 'divider', borderRadius: 2, fontSize: '0.9rem', bgcolor: 'action.hover' } }
        : { component: Box, sx: { py: 1, borderColor: 'divider', fontSize: '0.9rem' } };

    return (
        <>
            {showReasoning && (
                <ReasoningPanel text={reasoningText} open={reasoningOpen} onToggle={onToggleReasoning} />
            )}
            <Box {...(containerProps as any)}>
                <MarkdownErrorBoundary>
                    <MarkdownRenderer>{filterAttachments(msg.content)}</MarkdownRenderer>
                </MarkdownErrorBoundary>
            </Box>
        </>
    );
}, (prev, next) => {
    // Re-render only if content or relevant flags change for this item
    return (
        prev.msg === next.msg &&
        prev.busy === next.busy &&
        prev.messagesCount === next.messagesCount &&
        prev.showReasoning === next.showReasoning &&
        prev.reasoningText === next.reasoningText &&
        prev.reasoningOpen === next.reasoningOpen
    );
});

function MessageListComponent({ messages, busy, lastUserIdx, reasoningText, reasoningOpen, onToggleReasoning, messagesEndRef }: Props) {
    const count = messages.length;
    return (
        <Stack spacing={2}>
            {messages.map((msg: ChatMessage, index: number) => {
                const showReasoning = index === lastUserIdx || (index === 0 && lastUserIdx < 0 && !!reasoningText);
                return (
                    <MessageItem
                        key={msg.id || index}
                        msg={msg}
                        index={index}
                        messagesCount={count}
                        busy={busy}
                        showReasoning={showReasoning}
                        reasoningText={reasoningText}
                        reasoningOpen={reasoningOpen}
                        onToggleReasoning={onToggleReasoning}
                    />
                );
            })}
            <div ref={messagesEndRef} />
        </Stack>
    );
}

export default React.memo(MessageListComponent, (prev, next) => {
    return (
        prev.messages === next.messages &&
        prev.busy === next.busy &&
        prev.lastUserIdx === next.lastUserIdx &&
        prev.reasoningText === next.reasoningText &&
        prev.reasoningOpen === next.reasoningOpen &&
        prev.messagesEndRef === next.messagesEndRef
    );
});


