import React, { useState, useEffect, useRef, ReactElement, useMemo } from 'react';
import { EventsOn, LogDebug, LogInfo } from '../wailsjs/runtime/runtime';
import { SendUserMessage, Approve, GetTools, SetModel, GetSettings, SaveSettings, SetWorkspace, ClearConversation, GetConversations, LoadConversation, NewConversation } from '../wailsjs/go/bridge/App';
import * as Bridge from '../wailsjs/go/bridge/App';
import * as AppBridge from '../wailsjs/go/bridge/App';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import remarkBreaks from 'remark-breaks';
import { PrismLight as SyntaxHighlighter } from 'react-syntax-highlighter';
import { oneLight as oneLightStyle } from 'react-syntax-highlighter/dist/esm/styles/prism';
import ModelSelector from './ModelSelector';
import {
    Box,
    Stack,
    Typography,
    Paper,
    Divider,
    List,
    ListItem,
    ListItemText,
    Chip,
    TextField,
    Button,
    Dialog,
    DialogTitle,
    DialogContent,
    DialogActions,
    IconButton,
    Tooltip,
    Table as MuiTable,
    TableCell as MuiTableCell,
    TableRow as MuiTableRow,
    TableContainer as MuiTableContainer,
    Accordion,
    AccordionSummary,
    AccordionDetails,
    LinearProgress,
} from '@mui/material';
import ExpandMoreIcon from '@mui/icons-material/ExpandMore';
import SendIcon from '@mui/icons-material/Send';
import SettingsIcon from '@mui/icons-material/Settings';
import RuleIcon from '@mui/icons-material/Rule';

// Custom table components using MUI Table APIs
const CustomTable = ({ children }: any) => (
    <MuiTableContainer component={Paper} variant="outlined" sx={{ borderRadius: 1 }}>
        <MuiTable size="small">{children}</MuiTable>
    </MuiTableContainer>
)

const CustomTableRow = ({ children, ...props }: any) => (
    <MuiTableRow {...props}>{children}</MuiTableRow>
)

const CustomTableCell = ({ children, ...props }: any) => (
    <MuiTableCell {...props}>{children}</MuiTableCell>
)

const CustomTableHeader = ({ children, ...props }: any) => (
    <MuiTableCell {...props} component="th">
        {children}
    </MuiTableCell>
)

// Error boundary for ReactMarkdown
class MarkdownErrorBoundary extends React.Component<
    { children: React.ReactNode },
    { hasError: boolean; error?: Error }
> {
    constructor(props: { children: React.ReactNode }) {
        super(props);
        this.state = { hasError: false };
    }

    static getDerivedStateFromError(error: Error) {
        return { hasError: true, error };
    }

    componentDidCatch(error: Error, errorInfo: React.ErrorInfo) {
        console.error('Markdown rendering error:', error, errorInfo);
    }

    render() {
        if (this.state.hasError) {
            return (
                <div className="markdown-error">
                    <p>Failed to render markdown content:</p>
                    <pre>{this.state.error?.message}</pre>
                    <details>
                        <summary>Raw content</summary>
                        <pre style={{ whiteSpace: 'pre-wrap', fontSize: '0.8em' }}>
                            {typeof this.props.children === 'string' ? this.props.children : 'Content not available'}
                        </pre>
                    </details>
                </div>
            );
        }

        return this.props.children;
    }
}

// Helper function to format diff output with MUI components
const formatDiff = (diff: string): ReactElement => {
    if (!diff) return (
        <Typography variant="body2" sx={{ fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace' }}>
            No changes
        </Typography>
    )

    const lines = diff.split('\n')
    let inHeader = true
    const headerLines: string[] = []
    const contentLines: string[] = []

    for (const line of lines) {
        if (inHeader && (line.startsWith('---') || line.startsWith('+++'))) {
            headerLines.push(line)
        } else if (line === '') {
            inHeader = false
        } else {
            contentLines.push(line)
        }
    }

    return (
        <Box>
            {headerLines.length > 0 && (
                <Box sx={{ color: 'text.secondary', mb: 1 }}>
                    {headerLines.map((line, i) => (
                        <Typography
                            key={`header-${i}`}
                            variant="caption"
                            component="div"
                            sx={{ fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace' }}
                        >
                            {line}
                        </Typography>
                    ))}
                </Box>
            )}
            <Box>
                {contentLines.map((line, i) => {
                    if (line.startsWith('+')) {
                        const match = line.match(/^(\+)(\s*\d+:\s)(.*)$/)
                        return (
                            <Box
                                key={`line-${i}`}
                                sx={{
                                    bgcolor: 'success.light',
                                    color: 'success.contrastText',
                                    px: 1,
                                    py: 0.25,
                                    borderRadius: 0.5,
                                    my: 0.25,
                                    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
                                }}
                            >
                                {match ? (
                                    <>
                                        <Box component="span" sx={{ opacity: 0.75, mr: 1 }}>
                                            {match[2]}
                                        </Box>
                                        {match[3]}
                                    </>
                                ) : (
                                    line
                                )}
                            </Box>
                        )
                    }

                    if (line.startsWith('-')) {
                        const match = line.match(/^(\-)(\s*\d+:\s)(.*)$/)
                        return (
                            <Box
                                key={`line-${i}`}
                                sx={{
                                    bgcolor: 'error.light',
                                    color: 'error.contrastText',
                                    px: 1,
                                    py: 0.25,
                                    borderRadius: 0.5,
                                    my: 0.25,
                                    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
                                }}
                            >
                                {match ? (
                                    <>
                                        <Box component="span" sx={{ opacity: 0.75, mr: 1 }}>
                                            {match[2]}
                                        </Box>
                                        {match[3]}
                                    </>
                                ) : (
                                    line
                                )}
                            </Box>
                        )
                    }

                    if (line.match(/^\d+ line\(s\) changed$/)) {
                        return (
                            <Typography key={`line-${i}`} variant="caption" color="text.secondary">
                                {line}
                            </Typography>
                        )
                    }

                    const match = line.match(/^(\s)(\s*\d+:\s)(.*)$/)
                    if (match) {
                        return (
                            <Box key={`line-${i}`} sx={{ fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace' }}>
                                <Box component="span" sx={{ opacity: 0.5, mr: 1 }}>
                                    {match[2]}
                                </Box>
                                {match[3]}
                            </Box>
                        )
                    }

                    return (
                        <Box key={`line-${i}`} sx={{ fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace' }}>
                            {line}
                        </Box>
                    )
                })}
            </Box>
        </Box>
    )
}

// Define types for messages from backend
interface ChatMessage {
    role: string;
    content: string;
    id?: string;
}

interface ApprovalRequest {
    id: string;
    summary: string;
    diff: string;
}

interface Tool {
    name: string;
    description: string;
    safe: boolean;
}

const App: React.FC = () => {
    const [messages, setMessages] = useState<ChatMessage[]>([]);
    const [input, setInput] = useState('');
    const [approvalRequest, setApprovalRequest] = useState<ApprovalRequest | null>(null);
    const [tools, setTools] = useState<Tool[]>([]);
    const [currentModel, setCurrentModel] = useState<string>('');
    const messagesEndRef = useRef<null | HTMLDivElement>(null);
    const [busy, setBusy] = useState<boolean>(false);
    const [settingsOpen, setSettingsOpen] = useState<boolean>(false);
    const [workspaceOpen, setWorkspaceOpen] = useState<boolean>(false);
    const [workspacePath, setWorkspacePath] = useState<string>('');
    const [openaiKey, setOpenaiKey] = useState<string>('');
    const [anthropicKey, setAnthropicKey] = useState<string>('');
    const [ollamaEndpoint, setOllamaEndpoint] = useState<string>('');
    const [autoApproveShell, setAutoApproveShell] = useState<boolean>(false);
    const [autoApproveEdits, setAutoApproveEdits] = useState<boolean>(false);
    const [rulesOpen, setRulesOpen] = useState<boolean>(false);
    const [userRules, setUserRules] = useState<string[]>([]);
    const [projectRules, setProjectRules] = useState<string[]>([]);
    const [newUserRule, setNewUserRule] = useState<string>('');
    const [newProjectRule, setNewProjectRule] = useState<string>('');
    const [conversations, setConversations] = useState<{ id: string; title: string; updated_at?: string }[]>([]);
    const [currentConversationId, setCurrentConversationId] = useState<string>('');
    const [reasoningText, setReasoningText] = useState<string>('');
    const [reasoningOpen, setReasoningOpen] = useState<boolean>(false);
    const collapseTimerRef = useRef<number | null>(null);

    const orderedConversations = useMemo(
        () => {
            if (!currentConversationId) return conversations;
            const idx = conversations.findIndex(c => c.id === currentConversationId);
            if (idx < 0) return conversations;
            const current = conversations[idx];
            const rest = conversations.filter((_, i) => i !== idx);
            return [current, ...rest];
        },
        [conversations, currentConversationId]
    );

    // Track the index of the last user message to anchor the reasoning panel before the assistant reply
    const lastUserIdx = useMemo(() => {
        for (let i = messages.length - 1; i >= 0; i--) {
            if (messages[i]?.role === 'user') return i;
        }
        return -1;
    }, [messages]);

    useEffect(() => {
        // Listen for new chat messages
        EventsOn('chat:new', (message: ChatMessage) => {
            setMessages((prev: ChatMessage[]) => [...prev, message]);
        });

        // Listen for streaming assistant messages (final output only)
        EventsOn('assistant-msg', (content: string) => {
            try { LogDebug(`[UI] assistant-msg len=${content?.length ?? 0}`) } catch { }
            setMessages((prev: ChatMessage[]) => {
                const lastMessage = prev[prev.length - 1];
                if (lastMessage && lastMessage.role === 'assistant') {
                    return [
                        ...prev.slice(0, -1),
                        { ...lastMessage, content: content || '' }
                    ];
                }
                return [
                    ...prev,
                    { role: 'assistant', content: content || '' }
                ];
            });
        });

        // Listen for explicit reasoning stream
        EventsOn('assistant-reasoning', (payload: any) => {
            const text = String(payload?.text || '');
            const done = Boolean(payload?.done);
            if (!text && !done) return;
            setReasoningText((prev: string) => {
                if (!text) return prev;
                const prior = prev || '';
                return prior + text;
            });
            if (done) {
                setReasoningOpen(true);
                if (collapseTimerRef.current) { clearTimeout(collapseTimerRef.current); collapseTimerRef.current = null; }
                collapseTimerRef.current = window.setTimeout(() => {
                    setReasoningOpen(false);
                    collapseTimerRef.current = null;
                }, 1200);
            } else {
                if (collapseTimerRef.current) { clearTimeout(collapseTimerRef.current); collapseTimerRef.current = null; }
                setReasoningOpen(true);
            }
        });

        // Listen for clear chat event to reset UI state and refresh conversation list
        EventsOn('chat:clear', () => {
            try { LogInfo('[UI] chat:clear received; resetting UI state') } catch { }
            setMessages([]);
            setReasoningText('');
            setReasoningOpen(false);
            if (collapseTimerRef.current) {
                clearTimeout(collapseTimerRef.current);
                collapseTimerRef.current = null;
            }
            GetConversations()
                .then((res: any) => {
                    setCurrentConversationId(res?.current_id || '');
                    const list = Array.isArray(res?.conversations) ? res.conversations : [];
                    setConversations(list.map((c: any) => ({ id: String(c.id), title: String(c.title || c.id), updated_at: String(c.updated_at || '') })));
                })
                .catch(() => { });
        });

        // Listen for approval requests
        EventsOn('task:prompt', (request: ApprovalRequest) => {
            setApprovalRequest(request);
        });

        // Listen for busy state changes
        EventsOn('system:busy', (isBusy: boolean) => {
            setBusy(!!isBusy);
        });

        // Get available tools
        GetTools().then((fetchedTools: Record<string, any>[]) => {
            const typedTools: Tool[] = fetchedTools.map(tool => ({
                name: tool.name || '',
                description: tool.description || '',
                safe: Boolean(tool.safe)
            }));
            setTools(typedTools);
        });

        // Load settings
        GetSettings()
            .then((s: any) => {
                setOpenaiKey(s?.openai_api_key || '');
                setAnthropicKey(s?.anthropic_api_key || '');
                setOllamaEndpoint(s?.ollama_endpoint || '');
                setAutoApproveShell(String(s?.auto_approve_shell).toLowerCase() === 'true');
                setAutoApproveEdits(String(s?.auto_approve_edits).toLowerCase() === 'true');
                const last = s?.last_workspace || '';
                if (last) {
                    setWorkspacePath(last);
                    SetWorkspace(last)
                        .then(() => {
                            return AppBridge.GetRules();
                        })
                        .then((r: any) => {
                            setUserRules(Array.isArray(r?.user) ? r.user : []);
                            setProjectRules(Array.isArray(r?.project) ? r.project : []);
                        })
                        .then(() => NewConversation())
                        .then((id: string) => {
                            setCurrentConversationId(id);
                        })
                        .catch(() => { });
                } else {
                    setWorkspaceOpen(true);
                }

                // Preselect last selected model if available
                const lastModel = s?.last_model || '';
                if (lastModel) {
                    setCurrentModel(lastModel);
                    // Inform backend to set model immediately on startup
                    SetModel(lastModel).catch(() => { });
                }
            })
            .catch(() => { });

        // Load conversations list
        GetConversations()
            .then((res: any) => {
                setCurrentConversationId(res?.current_id || '');
                const list = Array.isArray(res?.conversations) ? res.conversations : [];
                setConversations(list.map((c: any) => ({ id: String(c.id), title: String(c.title || c.id), updated_at: String(c.updated_at || '') })));
            })
            .catch(() => { });

        // Load rules
        AppBridge.GetRules()
            .then((r: any) => {
                setUserRules(Array.isArray(r?.user) ? r.user : []);
                setProjectRules(Array.isArray(r?.project) ? r.project : []);
            })
            .catch(() => { });
    }, []);

    // Scroll to bottom when messages change
    useEffect(() => {
        messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
    }, [messages]);

    const handleSend = () => {
        if (!input.trim() || busy) return;

        // Send message to backend
        SendUserMessage(input);
        setInput('');
    };

    const handleApproval = (approved: boolean) => {
        if (approvalRequest) {
            Approve(approvalRequest.id, approved);
            setApprovalRequest(null);
        }
    };

    // Handle model selection
    const handleModelSelect = (model: string) => {
        setCurrentModel(model);
        SetModel(model).catch(err => {
            console.error("Failed to set model:", err);
        });
    };

    const handleSelectConversation = (id: string) => {
        if (!id || id === currentConversationId) return;
        setCurrentConversationId(id);
        LoadConversation(id).catch(() => { });
    };

    const handleNewConversation = () => {
        NewConversation()
            .then((id: string) => {
                setCurrentConversationId(id);
                // Prepend a placeholder until next refresh
                setConversations((prev: { id: string; title: string; updated_at?: string }[]) => [{ id, title: 'New Conversation' }, ...prev]);
            })
            .catch(() => { });
    };

    return (
        <Box display="flex" height="100vh" sx={{ bgcolor: 'background.default' }}>
            {/* Sidebar */}
            <Box
                sx={{
                    width: 320,
                    borderRight: 1,
                    borderColor: 'divider',
                    px: 3,
                    py: 3,
                    display: 'flex',
                    flexDirection: 'column',
                    gap: 3,
                    overflowY: 'auto',
                }}
            >
                <Box sx={{ pt: 2 }}>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.5 }}>
                        <Box component="img" src="/logo.png" alt="Loom" sx={{ width: 28, height: 28, borderRadius: 0.5 }} />
                        <Typography variant="h6" fontWeight={600}>
                            Loom v2
                        </Typography>
                        <Box sx={{ flex: 1 }} />
                        <Tooltip title="Select Workspace">
                            <IconButton size="small" onClick={() => setWorkspaceOpen(true)}>
                                <Typography variant="caption">WS</Typography>
                            </IconButton>
                        </Tooltip>
                        <Tooltip title="Rules">
                            <IconButton size="small" onClick={() => setRulesOpen(true)}>
                                <RuleIcon fontSize="small" />
                            </IconButton>
                        </Tooltip>
                        <Tooltip title="Settings">
                            <IconButton size="small" onClick={() => setSettingsOpen(true)}>
                                <SettingsIcon fontSize="small" />
                            </IconButton>
                        </Tooltip>
                    </Box>
                    <Typography variant="body2" color="text.secondary">
                        Minimal, calm, precise.
                    </Typography>
                </Box>

                <Box>
                    <ModelSelector onSelect={handleModelSelect} currentModel={currentModel} />
                </Box>

                <Box>
                    <Typography variant="subtitle2" sx={{ mb: 1, display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                        <span>Conversations</span>
                        <Button size="small" variant="outlined" onClick={handleNewConversation}>New</Button>
                    </Typography>
                    <Paper variant="outlined" sx={{ p: 1, mb: 2 }}>
                        <List dense sx={{ width: '100%' }}>
                            {orderedConversations.map((c: { id: string; title: string; updated_at?: string }) => (
                                <ListItem
                                    key={c.id}
                                    disableGutters
                                    onClick={() => handleSelectConversation(c.id)}
                                    sx={{ cursor: 'pointer', bgcolor: c.id === currentConversationId ? 'action.selected' : 'transparent', borderRadius: 0.5, px: 1 }}
                                >
                                    <ListItemText
                                        primary={c.title || c.id}
                                        secondary={c.updated_at ? new Date(c.updated_at).toLocaleString() : undefined}
                                        primaryTypographyProps={{ fontWeight: c.id === currentConversationId ? 700 : 500 }}
                                    />
                                </ListItem>
                            ))}
                            {conversations.length === 0 && (
                                <ListItem disableGutters>
                                    <ListItemText primary="No conversations yet" />
                                </ListItem>
                            )}
                        </List>
                    </Paper>
                </Box>

                <Box>
                    <Typography variant="subtitle2" sx={{ mb: 1 }}>
                        Tools ({tools.length})
                    </Typography>
                    <Paper variant="outlined" sx={{ p: 1 }}>
                        <List dense sx={{ width: '100%' }}>
                            {tools.map((tool: Tool) => (
                                <ListItem key={tool.name} disableGutters secondaryAction={
                                    <Chip
                                        size="small"
                                        label={tool.safe ? 'Safe' : 'Approval'}
                                        color={tool.safe ? 'success' : 'warning'}
                                        variant={tool.safe ? 'outlined' : 'filled'}
                                    />
                                }>
                                    <ListItemText
                                        primary={tool.name}
                                        secondary={tool.description}
                                        primaryTypographyProps={{ fontWeight: 600 }}
                                    />
                                </ListItem>
                            ))}
                        </List>
                    </Paper>
                </Box>
            </Box>

            {/* Main */}
            <Box sx={{ flex: 1, display: 'flex', flexDirection: 'column', minWidth: 0, height: '100vh' }}>
                {/* Chat */}
                <Box sx={{ flex: 1, overflowY: 'auto', px: 4, py: 3, minHeight: 0 }}>
                    <Stack spacing={2}>
                        {messages.map((msg: ChatMessage, index: number) => {
                            const isUser = msg.role === 'user'
                            const containerProps = isUser
                                ? { component: Box, sx: { p: 2, border: '0.1px solid #cccccc', borderRadius: 2, } }
                                : { component: Box, sx: { py: 1, borderTop: '0.1px solid #eeeeee', borderBottom: '0.1px solid #eeeeee' } }

                            return (
                                <Box key={index} {...(containerProps as any)}>
                                    {/* Reasoning panel shown after the last user message and before the assistant reply */}
                                    {index === lastUserIdx && reasoningText && (
                                        <Box sx={{ mb: 1 }}>
                                            <Accordion
                                                expanded={reasoningOpen}
                                                onChange={(_, exp) => setReasoningOpen(exp)}
                                                sx={{ boxShadow: 'none', bgcolor: 'transparent', '&:before': { display: 'none' } }}
                                            >
                                                <AccordionSummary expandIcon={<ExpandMoreIcon />}>
                                                    <Typography variant="subtitle2" fontWeight={600}>
                                                        Planning / Reasoning
                                                    </Typography>
                                                </AccordionSummary>
                                                <AccordionDetails>
                                                    {reasoningOpen && (
                                                        <Box sx={{ mb: 1 }}>
                                                            <LinearProgress />
                                                        </Box>
                                                    )}
                                                    <MarkdownErrorBoundary>
                                                        <Box sx={{ color: 'text.secondary' }}>
                                                            <ReactMarkdown
                                                                remarkPlugins={[remarkGfm, remarkBreaks]}
                                                                components={{
                                                                    code({ node, inline, className, children, ...props }: any) {
                                                                        const match = /language-(\w+)/.exec(className || '')
                                                                        return !inline && match ? (
                                                                            <SyntaxHighlighter
                                                                                style={oneLightStyle as any}
                                                                                language={match[1]}
                                                                                PreTag="div"
                                                                            >
                                                                                {String(children).replace(/\n$/, '')}
                                                                            </SyntaxHighlighter>
                                                                        ) : (
                                                                            <Box
                                                                                component="code"
                                                                                className={className}
                                                                                sx={{
                                                                                    bgcolor: 'action.hover',
                                                                                    borderRadius: 1,
                                                                                    px: 0.5,
                                                                                    py: 0.25,
                                                                                    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
                                                                                }}
                                                                                {...props}
                                                                            >
                                                                                {children}
                                                                            </Box>
                                                                        )
                                                                    },
                                                                    table: CustomTable,
                                                                    tr: CustomTableRow,
                                                                    td: CustomTableCell,
                                                                    th: CustomTableHeader,
                                                                }}
                                                            >
                                                                {reasoningText}
                                                            </ReactMarkdown>
                                                        </Box>
                                                    </MarkdownErrorBoundary>
                                                </AccordionDetails>
                                            </Accordion>
                                        </Box>
                                    )}
                                    <MarkdownErrorBoundary>
                                        <ReactMarkdown
                                            remarkPlugins={[remarkGfm, remarkBreaks]}
                                            components={{
                                                code({ node, inline, className, children, ...props }: any) {
                                                    const match = /language-(\w+)/.exec(className || '')
                                                    return !inline && match ? (
                                                        <SyntaxHighlighter
                                                            style={oneLightStyle as any}
                                                            language={match[1]}
                                                            PreTag="div"
                                                        >
                                                            {String(children).replace(/\n$/, '')}
                                                        </SyntaxHighlighter>
                                                    ) : (
                                                        <Box
                                                            component="code"
                                                            className={className}
                                                            sx={{
                                                                bgcolor: 'action.hover',
                                                                borderRadius: 1,
                                                                px: 0.5,
                                                                py: 0.25,
                                                                fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
                                                            }}
                                                            {...props}
                                                        >
                                                            {children}
                                                        </Box>
                                                    )
                                                },
                                                table: CustomTable,
                                                tr: CustomTableRow,
                                                td: CustomTableCell,
                                                th: CustomTableHeader,
                                            }}
                                        >
                                            {msg.content}
                                        </ReactMarkdown>
                                    </MarkdownErrorBoundary>
                                </Box>
                            )
                        })}
                        <div ref={messagesEndRef} />
                    </Stack>
                </Box>

                {/* Composer */}
                <Divider />
                <Box sx={{ px: 3, py: 2 }}>
                    <Stack direction="row" spacing={1} alignItems="flex-end">
                        <TextField
                            value={input}
                            onChange={(e) => setInput(e.target.value)}
                            onKeyDown={(e) => {
                                if (e.key === 'Enter' && !e.shiftKey) {
                                    e.preventDefault()
                                    if (!busy) handleSend()
                                }
                            }}
                            placeholder="Ask Loom anything…"
                            disabled={busy}
                            multiline
                            minRows={1}
                            maxRows={8}
                            fullWidth
                        />
                        <Button
                            onClick={() => {
                                // clear UI immediately, then ask backend to clear memory
                                setMessages([])
                                ClearConversation()
                            }}
                            color="inherit"
                            disabled={busy}
                            variant="outlined"
                        >
                            Clear
                        </Button>
                        <Button
                            onClick={handleSend}
                            disabled={busy || !input.trim()}
                            variant="contained"
                            endIcon={<SendIcon />}
                        >
                            {busy ? 'Working…' : 'Send'}
                        </Button>
                    </Stack>
                </Box>
            </Box>

            {/* Approval Dialog */}
            <Dialog open={!!approvalRequest} onClose={() => setApprovalRequest(null)} maxWidth="md" fullWidth>
                <DialogTitle>Action Requires Approval</DialogTitle>
                <DialogContent dividers>
                    <Typography variant="subtitle1" sx={{ mb: 2 }}>
                        {approvalRequest?.summary}
                    </Typography>
                    <Box sx={{
                        bgcolor: 'background.paper',
                        p: 2,
                        borderRadius: 1,
                        border: '1px solid',
                        borderColor: 'divider',
                        overflow: 'auto',
                        maxHeight: 400,
                        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
                    }}>
                        {approvalRequest && formatDiff(approvalRequest.diff)}
                    </Box>
                </DialogContent>
                <DialogActions>
                    <Button color="inherit" onClick={() => handleApproval(false)}>Reject</Button>
                    <Button variant="contained" onClick={() => handleApproval(true)}>Approve</Button>
                </DialogActions>
            </Dialog>

            {/* Rules Dialog */}
            <Dialog open={rulesOpen} onClose={() => setRulesOpen(false)} maxWidth="md" fullWidth>
                <DialogTitle>Rules</DialogTitle>
                <DialogContent dividers>
                    <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2} sx={{ mt: 1 }}>
                        <Box sx={{ flex: 1 }}>
                            <Typography variant="subtitle2" sx={{ mb: 1 }}>User Rules (apply to all projects)</Typography>
                            <Paper variant="outlined" sx={{ p: 1 }}>
                                <Stack spacing={1}>
                                    {userRules.map((r: string, idx: number) => (
                                        <Stack key={`ur-${idx}`} direction="row" spacing={1} alignItems="center">
                                            <TextField
                                                size="small"
                                                fullWidth
                                                value={r}
                                                onChange={(e) => {
                                                    const next = [...userRules];
                                                    next[idx] = e.target.value;
                                                    setUserRules(next);
                                                }}
                                            />
                                            <Button color="inherit" onClick={() => setUserRules(userRules.filter((_: string, i: number) => i !== idx))}>Delete</Button>
                                        </Stack>
                                    ))}
                                    <Stack direction="row" spacing={1}>
                                        <TextField
                                            size="small"
                                            fullWidth
                                            placeholder="Add a new user rule"
                                            value={newUserRule}
                                            onChange={(e) => setNewUserRule(e.target.value)}
                                            onKeyDown={(e) => {
                                                if (e.key === 'Enter' && newUserRule.trim()) {
                                                    setUserRules([...userRules, newUserRule.trim()]);
                                                    setNewUserRule('');
                                                }
                                            }}
                                        />
                                        <Button
                                            variant="outlined"
                                            onClick={() => {
                                                if (newUserRule.trim()) {
                                                    setUserRules([...userRules, newUserRule.trim()]);
                                                    setNewUserRule('');
                                                }
                                            }}
                                        >Add</Button>
                                    </Stack>
                                </Stack>
                            </Paper>
                        </Box>
                        <Box sx={{ flex: 1 }}>
                            <Typography variant="subtitle2" sx={{ mb: 1 }}>Project Rules (saved in .loom/rules.json)</Typography>
                            <Paper variant="outlined" sx={{ p: 1 }}>
                                <Stack spacing={1}>
                                    {projectRules.map((r: string, idx: number) => (
                                        <Stack key={`pr-${idx}`} direction="row" spacing={1} alignItems="center">
                                            <TextField
                                                size="small"
                                                fullWidth
                                                value={r}
                                                onChange={(e) => {
                                                    const next = [...projectRules];
                                                    next[idx] = e.target.value;
                                                    setProjectRules(next);
                                                }}
                                            />
                                            <Button color="inherit" onClick={() => setProjectRules(projectRules.filter((_: string, i: number) => i !== idx))}>Delete</Button>
                                        </Stack>
                                    ))}
                                    <Stack direction="row" spacing={1}>
                                        <TextField
                                            size="small"
                                            fullWidth
                                            placeholder="Add a new project rule"
                                            value={newProjectRule}
                                            onChange={(e) => setNewProjectRule(e.target.value)}
                                            onKeyDown={(e) => {
                                                if (e.key === 'Enter' && newProjectRule.trim()) {
                                                    setProjectRules([...projectRules, newProjectRule.trim()]);
                                                    setNewProjectRule('');
                                                }
                                            }}
                                        />
                                        <Button
                                            variant="outlined"
                                            onClick={() => {
                                                if (newProjectRule.trim()) {
                                                    setProjectRules([...projectRules, newProjectRule.trim()]);
                                                    setNewProjectRule('');
                                                }
                                            }}
                                        >Add</Button>
                                    </Stack>
                                </Stack>
                            </Paper>
                        </Box>
                    </Stack>
                </DialogContent>
                <DialogActions>
                    <Button onClick={() => setRulesOpen(false)} color="inherit">Close</Button>
                    <Button
                        variant="contained"
                        onClick={() => {
                            AppBridge.SaveRules({ user: userRules, project: projectRules }).finally(() => setRulesOpen(false));
                        }}
                    >Save</Button>
                </DialogActions>
            </Dialog>
            {/* Settings Dialog */}
            <Dialog open={settingsOpen} onClose={() => setSettingsOpen(false)} maxWidth="sm" fullWidth>
                <DialogTitle>Settings</DialogTitle>
                <DialogContent dividers>
                    <Stack spacing={2} sx={{ mt: 1 }}>
                        <TextField
                            label="OpenAI API Key"
                            type="password"
                            autoComplete="off"
                            value={openaiKey}
                            onChange={(e) => setOpenaiKey(e.target.value)}
                            placeholder="sk-..."
                            fullWidth
                        />
                        <TextField
                            label="Anthropic API Key"
                            type="password"
                            autoComplete="off"
                            value={anthropicKey}
                            onChange={(e) => setAnthropicKey(e.target.value)}
                            placeholder="sk-ant-..."
                            fullWidth
                        />
                        <TextField
                            label="Ollama Endpoint"
                            value={ollamaEndpoint}
                            onChange={(e) => setOllamaEndpoint(e.target.value)}
                            placeholder="http://localhost:11434"
                            fullWidth
                        />
                        <Stack direction="row" spacing={2} alignItems="center">
                            <Tooltip title="If enabled, shell commands proposed by the model are executed without manual approval.">
                                <Chip label="Auto-Approve Shell" />
                            </Tooltip>
                            <Button variant={autoApproveShell ? 'contained' : 'outlined'} onClick={() => setAutoApproveShell(!autoApproveShell)}>
                                {autoApproveShell ? 'On' : 'Off'}
                            </Button>
                        </Stack>
                        <Stack direction="row" spacing={2} alignItems="center">
                            <Tooltip title="If enabled, file edits are applied without manual approval.">
                                <Chip label="Auto-Approve Edits" />
                            </Tooltip>
                            <Button variant={autoApproveEdits ? 'contained' : 'outlined'} onClick={() => setAutoApproveEdits(!autoApproveEdits)}>
                                {autoApproveEdits ? 'On' : 'Off'}
                            </Button>
                        </Stack>
                    </Stack>
                </DialogContent>
                <DialogActions>
                    <Button onClick={() => setSettingsOpen(false)} color="inherit">Close</Button>
                    <Button
                        variant="contained"
                        onClick={() => {
                            SaveSettings({
                                openai_api_key: openaiKey,
                                anthropic_api_key: anthropicKey,
                                ollama_endpoint: ollamaEndpoint,
                                auto_approve_shell: String(autoApproveShell),
                                auto_approve_edits: String(autoApproveEdits),
                            }).finally(() => setSettingsOpen(false));
                        }}
                    >
                        Save
                    </Button>
                </DialogActions>
            </Dialog>

            {/* Workspace Dialog */}
            <Dialog open={workspaceOpen} onClose={() => setWorkspaceOpen(false)} maxWidth="sm" fullWidth>
                <DialogTitle>Select Workspace</DialogTitle>
                <DialogContent dividers>
                    <Stack spacing={2} sx={{ mt: 1 }}>
                        <Stack direction="row" spacing={1}>
                            <TextField
                                label="Workspace Path"
                                value={workspacePath}
                                onChange={(e) => setWorkspacePath(e.target.value)}
                                placeholder="/path/to/project"
                                fullWidth
                            />
                            <Button
                                variant="outlined"
                                onClick={() => {
                                    Bridge.ChooseWorkspace().then((path: string) => {
                                        if (path) setWorkspacePath(path);
                                    });
                                }}
                            >Browse…</Button>
                        </Stack>
                        <Typography variant="body2" color="text.secondary">
                            Enter a project directory. Project rules will be stored in <code>.loom/rules.json</code> under this path.
                        </Typography>
                    </Stack>
                </DialogContent>
                <DialogActions>
                    <Button onClick={() => setWorkspaceOpen(false)} color="inherit">Cancel</Button>
                    <Button
                        variant="contained"
                        onClick={() => {
                            const p = workspacePath.trim();
                            if (p) {
                                SetWorkspace(p)
                                    .then(() => AppBridge.GetRules())
                                    .then((r: any) => {
                                        setUserRules(Array.isArray(r?.user) ? r.user : []);
                                        setProjectRules(Array.isArray(r?.project) ? r.project : []);
                                        return NewConversation();
                                    })
                                    .then((id: string) => {
                                        setCurrentConversationId(id);
                                        return GetConversations();
                                    })
                                    .then((res: any) => {
                                        setCurrentConversationId(res?.current_id || '');
                                        const list = Array.isArray(res?.conversations) ? res.conversations : [];
                                        setConversations(list.map((c: any) => ({ id: String(c.id), title: String(c.title || c.id), updated_at: String(c.updated_at || '') })));
                                    })
                                    .finally(() => setWorkspaceOpen(false));
                            }
                        }}
                        disabled={!workspacePath.trim()}
                    >Use</Button>
                </DialogActions>
            </Dialog>
        </Box>
    );
};

export default App;
