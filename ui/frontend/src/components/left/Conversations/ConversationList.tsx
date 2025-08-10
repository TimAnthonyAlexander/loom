import React from 'react';
import { List, ListItem, ListItemText } from '@mui/material';
import { ConversationListItem } from '../../../types/ui';

type Props = {
    conversations: ConversationListItem[];
    currentConversationId: string;
    onSelect: (id: string) => void;
};

function ConversationListComponent({ conversations, currentConversationId, onSelect }: Props) {
    return (
        <List dense sx={{ width: '100%' }}>
            {conversations.map((c) => (
                <ListItem
                    key={c.id}
                    disableGutters
                    onClick={() => onSelect(c.id)}
                    sx={{ cursor: 'pointer', bgcolor: c.id === currentConversationId ? 'action.selected' : 'transparent', borderRadius: 0.5, px: 1 }}
                >
                    <ListItemText
                        primary={c.title || c.id}
                        secondary={c.updated_at ? new Date(c.updated_at).toLocaleString() : undefined}
                        primaryTypographyProps={{
                            fontWeight: c.id === currentConversationId ? 700 : 500,
                            fontSize: '0.8rem',
                        }}
                        secondaryTypographyProps={{
                            color: 'text.secondary',
                            fontSize: '0.7rem',
                        }}
                    />
                </ListItem>
            ))}
            {conversations.length === 0 && (
                <ListItem disableGutters>
                    <ListItemText primary="No conversations yet" />
                </ListItem>
            )}
        </List>
    );
}

export default React.memo(ConversationListComponent);


