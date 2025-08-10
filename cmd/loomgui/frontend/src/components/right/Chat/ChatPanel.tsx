import React from 'react';
import { Box, Divider } from '@mui/material';
import MessageList from './MessageList';
import Composer from './Composer';
import { ChatMessage } from '../../../types/ui';

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
};

export default function ChatPanel(props: Props) {
  const { messages, busy, lastUserIdx, reasoningText, reasoningOpen, onToggleReasoning, input, setInput, onSend, onClear, messagesEndRef } = props;
  return (
    <Box sx={{ width: 420, display: 'flex', flexDirection: 'column', minWidth: 0, height: '100vh' }}>
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
      </Box>
      <Divider />
      <Box sx={{ px: 3, py: 2 }}>
        <Composer input={input} setInput={setInput} busy={busy} onSend={onSend} onClear={onClear} />
      </Box>
    </Box>
  );
}


