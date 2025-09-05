import React from 'react';
import { List, ListItem, ListItemButton, ListItemText, Box, Button } from '@mui/material';
import { ConversationListItem } from '../../../types/ui';

type Props = {
    conversations: ConversationListItem[];
    currentConversationId: string;
    onSelect: (id: string) => void;
};

function ConversationListComponent({ conversations, currentConversationId, onSelect }: Props) {
    const [visibleCount, setVisibleCount] = React.useState(5);
    const visible = React.useMemo(() => conversations.slice(0, visibleCount), [conversations, visibleCount]);
    const hasMore = visibleCount < conversations.length;

    return (
        <List dense disablePadding sx={{ width: '100%' }}>
            {visible.map((c) => {
                const selected = c.id === currentConversationId;
                return (
                    <ListItem key={c.id} sx={{ width: '100%' }}>
                        <ListItemButton
                            disableRipple
                            onClick={() => onSelect(c.id)}
                            selected={selected}
                            sx={{
                                px: 1,
                                py: 0.25,
                                minHeight: 32,
                                gap: 0.75,
                                '&:hover': { backgroundColor: 'transparent', transform: 'translateX(2px)' },
                                transition: 'transform 120ms ease',
                                '&.Mui-selected': { backgroundColor: 'transparent' },
                                '&.Mui-selected:hover': { backgroundColor: 'transparent' }
                            }}
                        >
                            <Box
                                sx={{
                                    width: 6,
                                    height: 6,
                                    borderRadius: '50%',
                                    flexShrink: 0,
                                    opacity: selected ? 1 : 0,
                                    bgcolor: 'primary.main',
                                    transition: 'opacity 120ms ease'
                                }}
                            />
                            <ListItemText
                                primary={c.title || c.id}
                                secondary={c.updated_at ? new Date(c.updated_at).toLocaleString() : undefined}
                                primaryTypographyProps={{
                                    fontWeight: selected ? 700 : 500,
                                    fontSize: '0.8rem',
                                    lineHeight: 1.2,
                                    color: selected ? 'primary.main' : 'text.primary',
                                    noWrap: true
                                }}
                                secondaryTypographyProps={{
                                    color: 'text.secondary',
                                    fontSize: '0.7rem',
                                    lineHeight: 1.2,
                                    noWrap: true
                                }}
                                sx={{ m: 0, '& .MuiListItemText-secondary': { mt: 0.25 } }}
                            />
                        </ListItemButton>
                    </ListItem>
                );
            })}

            {conversations.length === 0 && (
                <ListItem disableGutters sx={{ px: 1, py: 0.5 }}>
                    <ListItemText
                        primary="No conversations yet"
                        primaryTypographyProps={{ fontSize: '0.8rem', color: 'text.secondary' }}
                    />
                </ListItem>
            )}

            {hasMore && (
                <ListItem sx={{ justifyContent: 'left', py: 0.5 }}>
                    <Button
                        size="small"
                        variant="text"
                        onClick={() => setVisibleCount((n) => Math.min(n + 5, conversations.length))}
                        sx={{ textTransform: 'none', px: 1 }}
                    >
                        Show more
                    </Button>
                </ListItem>
            )}
        </List>
    );
}

export default React.memo(ConversationListComponent);
