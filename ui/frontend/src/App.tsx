import React, { useState, useEffect, useRef, useMemo, useCallback } from 'react';
import { EventsOn, LogInfo } from '../wailsjs/runtime/runtime';
import { SendUserMessage, Approve, SetModel, GetSettings, SaveSettings, SetWorkspace, ClearConversation, GetConversations, LoadConversation, NewConversation } from '../wailsjs/go/bridge/App';
import * as Bridge from '../wailsjs/go/bridge/App';
import * as AppBridge from '../wailsjs/go/bridge/App';
import { Box } from '@mui/material';
import Sidebar from './components/left/Sidebar';
import EditorPanel from './components/center/EditorPanel';
import ChatPanel from './components/right/Chat/ChatPanel';
import ApprovalDialog from './components/dialogs/ApprovalDialog';
import SettingsDialog from './components/dialogs/SettingsDialog';
import RulesDialog from './components/dialogs/RulesDialog';
import WorkspaceDialog from './components/dialogs/WorkspaceDialog';
import SearchDialog from './components/dialogs/SearchDialog';
import { ChatMessage, ApprovalRequest, UIFileEntry, UIListDirResult, ConversationListItem, EditorTabItem } from './types/ui';
import { guessLanguage } from './utils/language';
import { writeFile } from './services/files';

// Types moved to ./types/ui

const App: React.FC = () => {
    const [messages, setMessages] = useState<ChatMessage[]>([]);
    // Chat input lives inside ChatPanel to avoid global rerenders on keystrokes
    const [approvalRequest, setApprovalRequest] = useState<ApprovalRequest | null>(null);
    const [currentModel, setCurrentModel] = useState<string>('');
    const messagesEndRef = useRef<HTMLDivElement>(null);
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
    const [conversations, setConversations] = useState<ConversationListItem[]>([]);
    const [currentConversationId, setCurrentConversationId] = useState<string>('');
    const [reasoningText, setReasoningText] = useState<string>('');
    const [reasoningOpen, setReasoningOpen] = useState<boolean>(false);
    const collapseTimerRef = useRef<number | null>(null);
    const [searchOpen, setSearchOpen] = useState<boolean>(false);
    const [searchMode, setSearchMode] = useState<'files' | 'text'>('files');

    // File explorer state
    const [dirCache, setDirCache] = useState<Record<string, UIFileEntry[]>>({}); // key: dir path ('' for root)
    const [expandedDirs, setExpandedDirs] = useState<Record<string, boolean>>({});
    const [openTabs, setOpenTabs] = useState<EditorTabItem[]>([]);
    const [activeTab, setActiveTab] = useState<string>('');

    // Normalize workspace-relative paths so tab identity is consistent
    const normalizeWorkspaceRelPath = (p: string): string => {
        let s = (p || '').trim();
        if (!s) return '';
        // Convert backslashes to forward slashes
        s = s.replace(/\\/g, '/');
        // If absolute under current workspace, convert to relative
        const ws = (workspacePath || '').replace(/\\/g, '/').trim();
        if (ws) {
            const wsClean = ws.endsWith('/') ? ws.slice(0, -1) : ws;
            if (s === wsClean) s = '';
            else if (s.startsWith(wsClean + '/')) s = s.slice(wsClean.length + 1);
        }
        // Remove any leading './'
        while (s.startsWith('./')) s = s.slice(2);
        // Collapse duplicate slashes
        s = s.replace(/\/{2,}/g, '/');
        // Avoid lone root
        if (s === '/') s = '';
        return s;
    };

    const orderedConversations = useMemo(() => {
        if (!currentConversationId) return conversations;
        const idx = conversations.findIndex(c => c.id === currentConversationId);
        if (idx < 0) return conversations;
        const current = conversations[idx];
        const rest = conversations.filter((_, i) => i !== idx);
        return [current, ...rest];
    }, [conversations, currentConversationId]);

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
                            // Reset and reload file explorer for the new workspace
                            setDirCache({});
                            setExpandedDirs({});
                            setOpenTabs([]);
                            setActiveTab('');
                            return Bridge.ListWorkspaceDir('');
                        })
                        .then((res: any) => {
                            const r = res as UIListDirResult;
                            if (r && Array.isArray(r.entries)) {
                                setDirCache((prev) => ({ ...prev, [r.path || '']: r.entries }));
                                setExpandedDirs((prev) => ({ ...prev, [r.path || '']: true }));
                            }
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
        // Initial file tree
        Bridge.ListWorkspaceDir('')
            .then((res: any) => {
                const r = res as UIListDirResult;
                if (r && Array.isArray(r.entries)) {
                    setDirCache((prev) => ({ ...prev, [r.path || '']: r.entries }));
                    setExpandedDirs((prev) => ({ ...prev, [r.path || '']: true }));
                }
            })
            .catch(() => { });
    }, []);

    // Scroll to bottom when messages change
    useEffect(() => {
        if (messagesEndRef.current) {
            messagesEndRef.current.scrollIntoView({ behavior: 'smooth' });
        }
    }, [messages]);

    const handleSend = useCallback((text: string) => {
        if (!text.trim() || busy) return;
        SendUserMessage(text);
    }, [busy]);

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
            // eslint-disable-next-line no-console
            console.error('Failed to set model:', err);
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
                setConversations((prev: ConversationListItem[]) => [{ id, title: 'New Conversation' }, ...prev]);
            })
            .catch(() => { });
    };

    // File explorer helpers
    const loadDir = (path: string) => {
        const key = path || '';
        if (dirCache[key]) return;
        Bridge.ListWorkspaceDir(key)
            .then((res: any) => {
                const r = res as UIListDirResult;
                if (r && Array.isArray(r.entries)) {
                    setDirCache((prev) => ({ ...prev, [r.path || key]: r.entries }));
                }
            })
            .catch(() => { });
    };

    const reloadFile = useCallback((path: string) => {
        const normPath = normalizeWorkspaceRelPath(path);
        if (!normPath) return;
        Bridge.ReadWorkspaceFile(normPath)
            .then((res: any) => {
                const content = String(res?.content || '');
                const serverRev = String(res?.serverRev || '');
                const language = guessLanguage(normPath);
                setOpenTabs((prev) => prev.map((t) =>
                    t.path.toLowerCase() === normPath.toLowerCase()
                        ? { ...t, content, serverRev, language, isDirty: false }
                        : t
                ));
            })
            .catch(() => { });
    }, [workspacePath]);

    const toggleDir = useCallback((path: string) => {
        const key = path || '';
        setExpandedDirs((prev) => ({ ...prev, [key]: !prev[key] }));
        if (!dirCache[key]) loadDir(key);
    }, [dirCache]);

    // Update tab helper
    const onUpdateTab = useCallback((path: string, patch: Partial<EditorTabItem>) => {
        setOpenTabs((prev) => prev.map((x) => (x.path === path ? { ...x, ...patch } : x)));
    }, []);

    const openFile = useCallback((path: string, line?: number, column?: number) => {
        const normPath = normalizeWorkspaceRelPath(path);
        // Deduplicate existing tabs (case-insensitive)
        setOpenTabs((prev) => {
            const seen: Record<string, boolean> = {};
            const result: EditorTabItem[] = [];
            for (const t of prev) {
                const key = t.path.toLowerCase();
                if (!seen[key]) {
                    seen[key] = true;
                    result.push(t);
                }
            }
            return result;
        });
        const exists = openTabs.find((t) => t.path.toLowerCase() === normPath.toLowerCase());
        if (exists) {
            setActiveTab(exists.path);
            if (typeof line === 'number' && typeof column === 'number') {
                onUpdateTab(exists.path, { cursor: { line, column } });
            }
            // Always reload on explicit open to reflect any external edits
            reloadFile(exists.path);
            return;
        }
        Bridge.ReadWorkspaceFile(normPath)
            .then((res: any) => {
                const title = (normPath.split('/').pop() || normPath);
                const content = String(res?.content || '');
                const serverRev = String(res?.serverRev || '');
                const language = guessLanguage(normPath);
                setOpenTabs((prev) => {
                    const next = prev.filter((t) => t.path.toLowerCase() !== normPath.toLowerCase());
                    next.push({
                        path: normPath,
                        title,
                        content,
                        language,
                        isDirty: false,
                        version: 1,
                        serverRev,
                        cursor: typeof line === 'number' && typeof column === 'number' ? { line, column } : { line: 1, column: 1 },
                        scrollTop: 0,
                    });
                    return next;
                });
                setActiveTab(normPath);
            })
            .catch(() => { });
    }, [openTabs, workspacePath, onUpdateTab, reloadFile]);

    const closeTab = useCallback((path: string) => {
        setOpenTabs((prev) => {
            const filtered = prev.filter((t) => t.path !== path);
            setActiveTab((current) => (current === path ? (filtered.length ? filtered[filtered.length - 1].path : '') : current));
            return filtered;
        });
    }, []);


    // Save tab helper
    const onSaveTab = useCallback(async (path: string) => {
        const t = openTabs.find((x) => x.path === path);
        if (!t) return;
        try {
            const res = await writeFile(t.path, t.content, t.serverRev);
            setOpenTabs((prev) =>
                prev.map((x) => (x.path === path ? { ...x, isDirty: false, serverRev: String(res?.serverRev || '') } : x)),
            );
        } catch (e) {
            // eslint-disable-next-line no-console
            console.error('Save failed', e);
        }
    }, [openTabs]);

    // Directory tree rendering is handled by the Sidebar > FileExplorer component

    // Listen for backend requests to open a file (e.g., after read/edit tools)
    useEffect(() => {
        const handler = (payload: any) => {
            const p = normalizeWorkspaceRelPath(String(payload?.path || ''));
            if (!p) return;
            const exists = openTabs.find((t) => t.path.toLowerCase() === p.toLowerCase());
            if (exists) {
                // If already open (e.g., after an edit), reload content and focus the tab
                reloadFile(p);
                setActiveTab(exists.path);
            } else {
                openFile(p);
            }
        };
        EventsOn('workspace:open_file', handler);
    }, []);

    // Global shortcuts: Cmd+P (quick open files), Cmd+Shift+F (text search)
    useEffect(() => {
        const onKeyDown = (e: KeyboardEvent) => {
            const isMac = navigator.platform.toLowerCase().includes('mac');
            const cmd = isMac ? e.metaKey : e.ctrlKey;
            const key = e.key.toLowerCase();
            if (cmd && key === 'p' && !e.shiftKey) {
                e.preventDefault();
                setSearchMode('files');
                setSearchOpen(true);
            } else if (cmd && key === 'f' && e.shiftKey) {
                e.preventDefault();
                setSearchMode('text');
                setSearchOpen(true);
            }
        };
        window.addEventListener('keydown', onKeyDown);
        return () => window.removeEventListener('keydown', onKeyDown);
    }, []);

    return (
        <Box display="flex" height="100vh" sx={{ bgcolor: 'background.default' }}>
            {/* Left: Sidebar */}
            <Box sx={{ minWidth: 280, width: '14%', borderRight: 1, borderColor: 'divider' }}>
                <Sidebar
                    onOpenWorkspace={() => setWorkspaceOpen(true)}
                    onOpenRules={() => setRulesOpen(true)}
                    onOpenSettings={() => setSettingsOpen(true)}
                    dirCache={dirCache}
                    expandedDirs={expandedDirs}
                    onToggleDir={toggleDir}
                    onOpenFile={openFile}
                />
            </Box>

            {/* Center: Tabbed Editor */}
            <Box sx={{ flex: 1 }}>
                <EditorPanel
                    openTabs={openTabs}
                    activeTab={activeTab}
                    onChangeActiveTab={(p: string) => {
                        setActiveTab(p);
                        if (p) {
                            Bridge.ReadWorkspaceFile(p)
                                .then((res: any) => {
                                    const content = String(res?.content || '');
                                    const serverRev = String(res?.serverRev || '');
                                    const language = guessLanguage(p);
                                    setOpenTabs((prev) => prev.map((t) => t.path === p ? { ...t, content, serverRev, language, isDirty: false } : t));
                                })
                                .catch(() => { });
                        }
                    }}
                    onCloseTab={closeTab}
                    onUpdateTab={onUpdateTab}
                    onSaveTab={onSaveTab}
                />
            </Box>

            {/* Right: Chat */}
            <ChatPanel
                messages={messages}
                busy={busy}
                lastUserIdx={lastUserIdx}
                reasoningText={reasoningText}
                reasoningOpen={reasoningOpen}
                onToggleReasoning={setReasoningOpen}
                onSend={handleSend}
                onClear={() => { setMessages([]); ClearConversation(); }}
                messagesEndRef={messagesEndRef}
                onNewConversation={handleNewConversation}
                conversations={orderedConversations}
                currentConversationId={currentConversationId}
                onSelectConversation={handleSelectConversation}
                currentModel={currentModel}
                onSelectModel={handleModelSelect}
            />

            <ApprovalDialog
                open={!!approvalRequest}
                summary={approvalRequest?.summary}
                diff={approvalRequest?.diff}
                onApprove={() => handleApproval(true)}
                onReject={() => handleApproval(false)}
                onClose={() => setApprovalRequest(null)}
            />
            <RulesDialog
                open={rulesOpen}
                userRules={userRules}
                setUserRules={setUserRules}
                projectRules={projectRules}
                setProjectRules={setProjectRules}
                newUserRule={newUserRule}
                setNewUserRule={setNewUserRule}
                newProjectRule={newProjectRule}
                setNewProjectRule={setNewProjectRule}
                onSave={() => { AppBridge.SaveRules({ user: userRules, project: projectRules }).finally(() => setRulesOpen(false)); }}
                onClose={() => setRulesOpen(false)}
            />
            <SettingsDialog
                open={settingsOpen}
                openaiKey={openaiKey}
                setOpenaiKey={setOpenaiKey}
                anthropicKey={anthropicKey}
                setAnthropicKey={setAnthropicKey}
                ollamaEndpoint={ollamaEndpoint}
                setOllamaEndpoint={setOllamaEndpoint}
                autoApproveShell={autoApproveShell}
                setAutoApproveShell={setAutoApproveShell}
                autoApproveEdits={autoApproveEdits}
                setAutoApproveEdits={setAutoApproveEdits}
                onSave={() => { SaveSettings({ openai_api_key: openaiKey, anthropic_api_key: anthropicKey, ollama_endpoint: ollamaEndpoint, auto_approve_shell: String(autoApproveShell), auto_approve_edits: String(autoApproveEdits) }).finally(() => setSettingsOpen(false)); }}
                onClose={() => setSettingsOpen(false)}
            />
            <WorkspaceDialog
                open={workspaceOpen}
                workspacePath={workspacePath}
                setWorkspacePath={setWorkspacePath}
                onBrowse={() => { Bridge.ChooseWorkspace().then((path: string) => { if (path) setWorkspacePath(path); }); }}
                onUse={() => {
                    const p = workspacePath.trim();
                    if (p) {
                        SetWorkspace(p)
                            .then(() => {
                                // Reset and reload file explorer for the new workspace
                                setDirCache({});
                                setExpandedDirs({});
                                setOpenTabs([]);
                                setActiveTab('');
                                return Bridge.ListWorkspaceDir('');
                            })
                            .then((res: any) => {
                                const r = res as UIListDirResult;
                                if (r && Array.isArray(r.entries)) {
                                    setDirCache((prev) => ({ ...prev, [r.path || '']: r.entries }));
                                    setExpandedDirs((prev) => ({ ...prev, [r.path || '']: true }));
                                }
                                return AppBridge.GetRules();
                            })
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
                onClose={() => setWorkspaceOpen(false)}
            />
            <SearchDialog
                open={searchOpen}
                initialMode={searchMode}
                onClose={() => setSearchOpen(false)}
                onOpenFile={(p, line, col) => {
                    setSearchOpen(false);
                    if (p) openFile(p, line, col);
                }}
            />
        </Box>
    );
};

export default App;
