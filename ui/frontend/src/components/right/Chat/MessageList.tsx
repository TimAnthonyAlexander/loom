import { Box, Stack, CircularProgress } from '@mui/material';
import InfoOutlinedIcon from '@mui/icons-material/InfoOutlined';
import MarkdownRenderer from '../../markdown/MarkdownRenderer';
import MarkdownErrorBoundary from '../../markdown/MarkdownErrorBoundary';
import ReasoningPanel from './ReasoningPanel';
import { ChatMessage } from '../../../types/ui';

type Props = {
    messages: ChatMessage[];
    busy: boolean;
    lastUserIdx: number;
    reasoningText: string;
    reasoningOpen: boolean;
    onToggleReasoning: (open: boolean) => void;
    messagesEndRef: React.RefObject<HTMLDivElement>;
};

export default function MessageList({ messages, busy, lastUserIdx, reasoningText, reasoningOpen, onToggleReasoning, messagesEndRef }: Props) {
    return (
        <Stack spacing={2}>
            {messages.map((msg: ChatMessage, index: number) => {
                const isUser = msg.role === 'user';
                const isSystem = msg.role === 'system';
                const isLastMessage = index === messages.length - 1;
                const showSpinner = isSystem && isLastMessage && busy;

                if (isSystem) {
                    return (
                        <Box
                            key={index}
                            sx={{
                                p: 1.25,
                                borderRadius: 1.5,
                                border: '1px solid',
                                borderColor: 'divider',
                                bgcolor: 'background.paper',
                                color: 'text.secondary',
                            }}
                        >
                            <Stack direction="row" spacing={1.25} alignItems="flex-start">
                                <Box sx={{ pt: '3px' }}>
                                    {showSpinner ? (
                                        <CircularProgress size={14} thickness={5} />
                                    ) : (
                                        <InfoOutlinedIcon sx={{ fontSize: 16, color: 'text.disabled' }} />
                                    )}
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
                    : { component: Box, sx: { py: 1, borderTop: '1px solid', borderBottom: '1px solid', borderColor: 'divider', fontSize: '0.9rem' } };

                return (
                    <Box key={index} {...(containerProps as any)}>
                        {index === lastUserIdx && (
                            <ReasoningPanel text={reasoningText} open={reasoningOpen} onToggle={onToggleReasoning} />
                        )}
                        <MarkdownErrorBoundary>
                            <MarkdownRenderer>{msg.content}</MarkdownRenderer>
                        </MarkdownErrorBoundary>
                    </Box>
                );
            })}
            <div ref={messagesEndRef} />
        </Stack>
    );
}


