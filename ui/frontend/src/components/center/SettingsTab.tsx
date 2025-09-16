import React from 'react';
import {
    Box,
    Stack,
    TextField,
    Link,
    FormControl,
    InputLabel,
    Select,
    MenuItem,
    FormControlLabel,
    Switch,
    Typography,
    IconButton,
    InputAdornment,
    Paper,
    Checkbox,
    FormGroup,
    Chip,
    Divider,
    CircularProgress
} from '@mui/material';
import KeyIcon from '@mui/icons-material/VpnKey';
import Visibility from '@mui/icons-material/Visibility';
import VisibilityOff from '@mui/icons-material/VisibilityOff';
import LaunchIcon from '@mui/icons-material/Launch';
import RefreshIcon from '@mui/icons-material/Refresh';
import SearchIcon from '@mui/icons-material/Search';
import { AVAILABLE_THEMES } from '../../themes/themeConfig';
import { OpenProjectDataDir } from '../../../wailsjs/go/bridge/App';
import { ALL_AVAILABLE_MODELS, getAllModels, ModelOption } from '../../models';

type Props = {
    openaiKey: string;
    setOpenaiKey: (v: string) => void;
    anthropicKey: string;
    setAnthropicKey: (v: string) => void;
    openrouterKey: string;
    setOpenrouterKey: (v: string) => void;
    ollamaEndpoint: string;
    setOllamaEndpoint: (v: string) => void;
    autoApproveShell: boolean;
    setAutoApproveShell: (v: boolean) => void;
    autoApproveEdits: boolean;
    setAutoApproveEdits: (v: boolean) => void;
    currentTheme: string;
    setCurrentTheme: (v: string) => void;
    selectedModels: string[];
    setSelectedModels: (v: string[]) => void;
    onSave: () => void;
};

export default function SettingsTab({
    openaiKey,
    setOpenaiKey,
    anthropicKey,
    setAnthropicKey,
    openrouterKey,
    setOpenrouterKey,
    ollamaEndpoint,
    setOllamaEndpoint,
    autoApproveShell,
    setAutoApproveShell,
    autoApproveEdits,
    setAutoApproveEdits,
    currentTheme,
    setCurrentTheme,
    selectedModels,
    setSelectedModels,
    onSave
}: Props) {
    const [showOpenAI, setShowOpenAI] = React.useState(false);
    const [showAnthropic, setShowAnthropic] = React.useState(false);
    const [showOpenRouter, setShowOpenRouter] = React.useState(false);
    const [allModels, setAllModels] = React.useState<ModelOption[]>(ALL_AVAILABLE_MODELS);
    const [loadingModels, setLoadingModels] = React.useState(false);
    const [modelLoadError, setModelLoadError] = React.useState<string | null>(null);
    const [modelSearchQuery, setModelSearchQuery] = React.useState<string>('');
    const [visibleCounts, setVisibleCounts] = React.useState<Record<string, number>>({});
    const [activeSection, setActiveSection] = React.useState('Appearance');

    // Load all models including dynamic OpenRouter models
    React.useEffect(() => {
        async function loadAllModels() {
            setLoadingModels(true);
            setModelLoadError(null);
            try {
                const models = await getAllModels();
                setAllModels(models);
                setModelLoadError(null);
            } catch (error) {
                console.warn('Failed to load dynamic models, using static fallback:', error);
                setAllModels(ALL_AVAILABLE_MODELS);
                setModelLoadError('Failed to load current OpenRouter models. Using cached models.');
            } finally {
                setLoadingModels(false);
            }
        }

        loadAllModels();
    }, []);

    // Reset visible counts when search changes to avoid confusion
    React.useEffect(() => {
        if (modelSearchQuery.trim()) {
            setVisibleCounts({});
        }
    }, [modelSearchQuery]);

    // Filter and group models
    const filteredAndGroupedModels = React.useMemo(() => {
        // First filter models based on search query
        const filteredModels = allModels.filter(model => {
            if (!modelSearchQuery.trim()) return true;

            const query = modelSearchQuery.toLowerCase();
            return (
                model.name.toLowerCase().includes(query) ||
                model.provider.toLowerCase().includes(query) ||
                (model.group && model.group.toLowerCase().includes(query)) ||
                model.id.toLowerCase().includes(query)
            );
        });

        // Group models with special handling for FREE models
        const groups: Record<string, ModelOption[]> = {};

        for (const model of filteredModels) {
            // Check if this is a FREE OpenRouter model (both input and output cost $0)
            const isFreeOpenRouter = model.provider === 'openrouter' &&
                model.pricing &&
                model.pricing.input === 0 &&
                model.pricing.output === 0;

            const groupName = isFreeOpenRouter ? 'FREE' : (model.group || model.provider);

            if (!groups[groupName]) {
                groups[groupName] = [];
            }
            groups[groupName].push(model);
        }

        // Sort groups: FREE first, then others
        const sortedGroups = Object.entries(groups).sort(([a], [b]) => {
            if (a === 'FREE') return -1;
            if (b === 'FREE') return 1;
            // Keep original ordering for other groups
            const order = ['Flagship', 'Reasoning', 'Production', 'Local', 'OpenRouter'];
            const aIndex = order.indexOf(a);
            const bIndex = order.indexOf(b);
            if (aIndex !== -1 && bIndex !== -1) return aIndex - bIndex;
            if (aIndex !== -1) return -1;
            if (bIndex !== -1) return 1;
            return a.localeCompare(b);
        });

        return sortedGroups;
    }, [allModels, modelSearchQuery]);



    // Initialize visible counts for all groups
    React.useEffect(() => {
        const initialCounts: Record<string, number> = {};
        filteredAndGroupedModels.forEach(([groupName]) => {
            if (!(groupName in visibleCounts)) {
                initialCounts[groupName] = 6; // Show 10 models initially per group
            }
        });
        if (Object.keys(initialCounts).length > 0) {
            setVisibleCounts(prev => ({ ...prev, ...initialCounts }));
        }
    }, [filteredAndGroupedModels, visibleCounts]);

    // Function to show more models for a specific group
    const showMoreForGroup = (groupName: string) => {
        setVisibleCounts(prev => ({
            ...prev,
            [groupName]: (prev[groupName] || 6) + 6
        }));
    };

    // Auto-save when any setting changes
    React.useEffect(() => {
        onSave();
    }, [openaiKey, anthropicKey, openrouterKey, ollamaEndpoint, autoApproveShell, autoApproveEdits, currentTheme, selectedModels, onSave]);

    const SectionTitle = ({ children }: { children: React.ReactNode }) => (
        <Typography variant="h6" sx={{
            fontWeight: 600,
            mb: 2,
            color: 'text.primary',
            borderBottom: 1,
            borderColor: 'divider',
            pb: 1
        }}>
            {children}
        </Typography>
    );

    const keyAdornment = (
        <InputAdornment position="start">
            <KeyIcon fontSize="small" />
        </InputAdornment>
    );

    // Define navigation sections
    const sections = [
        { id: 'Appearance', label: 'Appearance', icon: '🎨' },
        { id: 'Automation', label: 'Automation', icon: '⚡' },
        { id: 'Available Models', label: 'Models', icon: '🤖' },
        { id: 'API Keys', label: 'API Credentials', icon: '🔑' },
        { id: 'Local Models', label: 'Ollama', icon: '💻' },
    ];

    return (
        <Box sx={{
            height: '100%',
            display: 'flex',
            bgcolor: 'background.default'
        }}>
            {/* Navigation Sidebar */}
            <Box sx={{
                width: 280,
                bgcolor: 'background.paper',
                borderRight: 1,
                borderColor: 'divider',
                p: 3,
                display: 'flex',
                flexDirection: 'column'
            }}>
                <Typography
                    variant="h5"
                    sx={{
                        fontWeight: 700,
                        mb: 3,
                        background: 'linear-gradient(45deg, currentColor, primary.main)',
                        backgroundClip: 'text',
                        WebkitBackgroundClip: 'text',
                        color: 'transparent'
                    }}
                >
                    Settings
                </Typography>

                <Stack spacing={1}>
                    {sections.map((section) => (
                        <Box
                            key={section.id}
                            onClick={() => setActiveSection(section.id)}
                            sx={{
                                p: 2,
                                borderRadius: 2,
                                cursor: 'pointer',
                                display: 'flex',
                                alignItems: 'center',
                                gap: 2,
                                transition: 'all 0.2s ease',
                                bgcolor: activeSection === section.id ? 'primary.main' : 'transparent',
                                color: activeSection === section.id ? 'primary.contrastText' : 'text.primary',
                                '&:hover': {
                                    bgcolor: activeSection === section.id ? 'primary.main' : 'action.hover',
                                    transform: 'translateX(4px)'
                                }
                            }}
                        >
                            <Typography variant="body1" sx={{ fontWeight: activeSection === section.id ? 600 : 500 }}>
                                {section.label}
                            </Typography>
                        </Box>
                    ))}
                </Stack>
            </Box>

            {/* Main Content Area */}
            <Box sx={{
                flex: 1,
                height: '100%',
                overflow: 'auto',
                p: 3
            }}>
                {/* API Keys Section */}
                {activeSection === 'API Keys' && (
                    <Paper elevation={0} sx={{ p: 3, border: 1, borderColor: 'divider', borderRadius: 2 }}>
                        <SectionTitle>API Keys</SectionTitle>
                        <Stack spacing={2.5}>
                            <TextField
                                label="OpenAI API Key"
                                type={showOpenAI ? 'text' : 'password'}
                                autoComplete="off"
                                value={openaiKey}
                                onChange={(e) => setOpenaiKey(e.target.value)}
                                placeholder="sk-..."
                                fullWidth
                                InputProps={{
                                    startAdornment: keyAdornment,
                                    endAdornment: (
                                        <InputAdornment position="end">
                                            <IconButton onClick={() => setShowOpenAI((v) => !v)} edge="end">
                                                {showOpenAI ? <VisibilityOff /> : <Visibility />}
                                            </IconButton>
                                        </InputAdornment>
                                    )
                                }}
                            />
                            <TextField
                                label="Anthropic API Key"
                                type={showAnthropic ? 'text' : 'password'}
                                autoComplete="off"
                                value={anthropicKey}
                                onChange={(e) => setAnthropicKey(e.target.value)}
                                placeholder="sk-ant-..."
                                fullWidth
                                InputProps={{
                                    startAdornment: keyAdornment,
                                    endAdornment: (
                                        <InputAdornment position="end">
                                            <IconButton onClick={() => setShowAnthropic((v) => !v)} edge="end">
                                                {showAnthropic ? <VisibilityOff /> : <Visibility />}
                                            </IconButton>
                                        </InputAdornment>
                                    )
                                }}
                            />
                            <TextField
                                label="OpenRouter API Key"
                                type={showOpenRouter ? 'text' : 'password'}
                                autoComplete="off"
                                value={openrouterKey}
                                onChange={(e) => setOpenrouterKey(e.target.value)}
                                placeholder="sk-or-..."
                                fullWidth
                                InputProps={{
                                    startAdornment: keyAdornment,
                                    endAdornment: (
                                        <InputAdornment position="end">
                                            <IconButton onClick={() => setShowOpenRouter((v) => !v)} edge="end">
                                                {showOpenRouter ? <VisibilityOff /> : <Visibility />}
                                            </IconButton>
                                        </InputAdornment>
                                    )
                                }}
                            />
                        </Stack>
                    </Paper>
                )}

                {/* Local Models Section */}
                {activeSection === 'Local Models' && (
                    <Paper elevation={0} sx={{ p: 3, border: 1, borderColor: 'divider', borderRadius: 2 }}>
                        <SectionTitle>Local Models</SectionTitle>
                        <Stack spacing={2.5}>
                            <TextField
                                label="Ollama Endpoint"
                                value={ollamaEndpoint}
                                onChange={(e) => setOllamaEndpoint(e.target.value)}
                                placeholder="http://localhost:11434"
                                fullWidth
                                InputProps={{ sx: { fontFamily: 'ui-monospace, Menlo, monospace' } }}
                            />
                        </Stack>
                    </Paper>
                )}

                {/* Appearance Section */}
                {activeSection === 'Appearance' && (
                    <Paper elevation={0} sx={{ p: 3, border: 1, borderColor: 'divider', borderRadius: 2 }}>
                        <SectionTitle>Appearance</SectionTitle>
                        <FormControl fullWidth>
                            <InputLabel shrink>Theme</InputLabel>
                            <Select
                                value={currentTheme}
                                label="Theme"
                                onChange={(e) => setCurrentTheme(e.target.value)}
                                MenuProps={{ PaperProps: { sx: { borderRadius: 2 } } }}
                            >
                                {Object.entries(AVAILABLE_THEMES).map(([key, config]) => (
                                    <MenuItem key={key} value={key} sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                                        {config.name}
                                    </MenuItem>
                                ))}
                            </Select>
                        </FormControl>
                    </Paper>
                )}

                {/* Available Models Section */}
                {activeSection === 'Available Models' && (
                    <Paper elevation={0} sx={{ p: 3, border: 1, borderColor: 'divider', borderRadius: 2 }}>
                        <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, mb: 2 }}>
                            <Typography variant="h6" sx={{
                                fontWeight: 600,
                                color: 'text.primary',
                                borderBottom: 1,
                                borderColor: 'divider',
                                pb: 1,
                                flex: 1
                            }}>
                                Available Models
                            </Typography>
                            <IconButton
                                size="small"
                                onClick={() => {
                                    // Reload models manually
                                    async function reloadModels() {
                                        setLoadingModels(true);
                                        setModelLoadError(null);
                                        try {
                                            const models = await getAllModels();
                                            setAllModels(models);
                                            setModelLoadError(null);
                                        } catch (error) {
                                            console.warn('Failed to reload dynamic models:', error);
                                            setModelLoadError('Failed to load current OpenRouter models. Using cached models.');
                                        } finally {
                                            setLoadingModels(false);
                                        }
                                    }
                                    reloadModels();
                                }}
                                disabled={loadingModels}
                                title="Refresh OpenRouter models"
                            >
                                <RefreshIcon fontSize="small" />
                            </IconButton>
                        </Box>
                        <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
                            Choose which models appear in the model selector dropdown. Only checked models will be available for selection.
                            <br />
                            OpenRouter models are loaded dynamically with current pricing information. Free models (0$ input and output) appear in the FREE section at the top.
                        </Typography>

                        {modelLoadError && (
                            <Box sx={{
                                bgcolor: 'warning.light',
                                color: 'warning.contrastText',
                                p: 2,
                                borderRadius: 1,
                                mb: 2,
                                border: 1,
                                borderColor: 'warning.main'
                            }}>
                                <Typography variant="body2">
                                    ⚠️ {modelLoadError}
                                </Typography>
                            </Box>
                        )}

                        <TextField
                            fullWidth
                            placeholder="Search models by name, provider, or group..."
                            value={modelSearchQuery}
                            onChange={(e) => setModelSearchQuery(e.target.value)}
                            onKeyDown={(e) => {
                                if (e.key === 'Escape') {
                                    setModelSearchQuery('');
                                }
                            }}
                            InputProps={{
                                startAdornment: (
                                    <InputAdornment position="start">
                                        <SearchIcon fontSize="small" />
                                    </InputAdornment>
                                ),
                                ...(modelSearchQuery && {
                                    endAdornment: (
                                        <InputAdornment position="end">
                                            <IconButton
                                                size="small"
                                                onClick={() => setModelSearchQuery('')}
                                                title="Clear search (Esc)"
                                            >
                                                <Typography variant="caption" sx={{ fontSize: '1rem' }}>✕</Typography>
                                            </IconButton>
                                        </InputAdornment>
                                    )
                                })
                            }}
                            sx={{ mb: 3 }}
                        />

                        <Stack spacing={3}>
                            {loadingModels ? (
                                <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, py: 4, justifyContent: 'center' }}>
                                    <CircularProgress size={20} />
                                    <Typography variant="body2" color="text.secondary">
                                        Loading OpenRouter models with current pricing...
                                    </Typography>
                                </Box>
                            ) : filteredAndGroupedModels.length === 0 ? (
                                <Box sx={{ py: 4, textAlign: 'center' }}>
                                    <Typography variant="body1" color="text.secondary" sx={{ mb: 1 }}>
                                        No models found matching "{modelSearchQuery}"
                                    </Typography>
                                    <Typography variant="body2" color="text.secondary">
                                        Try a different search term or clear the search to see all models.
                                    </Typography>
                                </Box>
                            ) : (
                                filteredAndGroupedModels.map(([groupName, groupModels]) => {
                                    // When searching, show all results without pagination
                                    const shouldPaginate = !modelSearchQuery.trim();
                                    const visibleCount = shouldPaginate ? (visibleCounts[groupName] || 6) : groupModels.length;
                                    const visibleModels = groupModels.slice(0, visibleCount);
                                    const hasMore = shouldPaginate && groupModels.length > visibleCount;

                                    return (
                                        <Box key={groupName}>
                                            <Typography variant="subtitle1" sx={{
                                                fontWeight: 600,
                                                mb: 1,
                                                color: groupName === 'FREE' ? 'success.main' : 'primary.main',
                                                display: 'flex',
                                                alignItems: 'center',
                                                gap: 1
                                            }}>
                                                {groupName === 'FREE' ? '🎉 FREE MODELS' : groupName}
                                                <Chip
                                                    label={`${groupModels.length} available`}
                                                    size="small"
                                                    color={groupName === 'FREE' ? 'success' : 'default'}
                                                    variant="outlined"
                                                    sx={{ fontSize: '0.7rem', height: 20 }}
                                                />
                                            </Typography>
                                            <FormGroup>
                                                {visibleModels.map((model) => (
                                                    <FormControlLabel
                                                        key={model.id}
                                                        control={
                                                            <Checkbox
                                                                checked={selectedModels.includes(model.id)}
                                                                onChange={(e) => {
                                                                    if (e.target.checked) {
                                                                        setSelectedModels([...selectedModels, model.id]);
                                                                    } else {
                                                                        setSelectedModels(selectedModels.filter(id => id !== model.id));
                                                                    }
                                                                }}
                                                                size="small"
                                                            />
                                                        }
                                                        label={
                                                            <Box>
                                                                <Typography variant="body2" sx={{ fontWeight: 500 }}>
                                                                    {model.name}
                                                                </Typography>
                                                                <Stack direction="row" spacing={1} alignItems="center" sx={{ mt: 0.5 }}>
                                                                    <Chip
                                                                        label={model.provider}
                                                                        size="small"
                                                                        variant="outlined"
                                                                        sx={{ fontSize: '0.7rem', height: 20 }}
                                                                    />
                                                                    {model.pricing && (
                                                                        <Typography
                                                                            variant="caption"
                                                                            color={groupName === 'FREE' ? 'success.main' : 'text.secondary'}
                                                                            sx={{
                                                                                fontWeight: groupName === 'FREE' ? 600 : 400
                                                                            }}
                                                                        >
                                                                            {groupName === 'FREE' ?
                                                                                '🎉 FREE - $0.00/M in • $0.00/M out' :
                                                                                `$${model.pricing.input?.toFixed(2)}/M in • $${model.pricing.output?.toFixed(2)}/M out`
                                                                            }
                                                                        </Typography>
                                                                    )}
                                                                </Stack>
                                                            </Box>
                                                        }
                                                        sx={{
                                                            alignItems: 'flex-start',
                                                            mb: 1,
                                                            '& .MuiFormControlLabel-label': {
                                                                flex: 1
                                                            },
                                                            ...(groupName === 'FREE' && {
                                                                border: 1,
                                                                borderColor: 'success.main',
                                                                borderRadius: 1,
                                                                bgcolor: 'transparent',
                                                            })
                                                        }}
                                                    />
                                                ))}
                                            </FormGroup>

                                            {hasMore && (
                                                <Box sx={{ mt: 2, mb: 1, textAlign: 'center' }}>
                                                    <Typography
                                                        variant="body2"
                                                        color="primary"
                                                        sx={{
                                                            cursor: 'pointer',
                                                            textDecoration: 'underline',
                                                            '&:hover': { opacity: 0.7 }
                                                        }}
                                                        onClick={() => showMoreForGroup(groupName)}
                                                    >
                                                        Show more ({groupModels.length - visibleCount} remaining)
                                                    </Typography>
                                                </Box>
                                            )}
                                        </Box>
                                    );
                                })
                            )}
                        </Stack>

                        <Divider sx={{ my: 2 }} />

                        <Stack direction="row" spacing={2} alignItems="center">
                            <Typography variant="body2" color="text.secondary">
                                Selected: {selectedModels.length} models
                                {modelSearchQuery ? (
                                    <span> • Found: {filteredAndGroupedModels.reduce((sum, [, models]) => sum + models.length, 0)} of {allModels.length} models</span>
                                ) : (
                                    <span> • Total: {allModels.length} available</span>
                                )}
                            </Typography>
                            <Box sx={{ flex: 1 }} />
                            <Typography
                                variant="body2"
                                color="primary"
                                sx={{ cursor: 'pointer', textDecoration: 'underline' }}
                                onClick={() => setSelectedModels(allModels.map(m => m.id))}
                            >
                                Select All
                            </Typography>
                            <Typography
                                variant="body2"
                                color="primary"
                                sx={{ cursor: 'pointer', textDecoration: 'underline' }}
                                onClick={() => setSelectedModels([])}
                            >
                                Clear All
                            </Typography>
                        </Stack>
                    </Paper>
                )}

                {/* Automation Section */}
                {activeSection === 'Automation' && (
                    <Paper elevation={0} sx={{ p: 3, border: 1, borderColor: 'divider', borderRadius: 2 }}>
                        <SectionTitle>Automation</SectionTitle>
                        <Stack spacing={2}>
                            <Box sx={{
                                p: 2,
                                bgcolor: 'action.hover',
                                borderRadius: 1,
                                border: 1,
                                borderColor: 'divider'
                            }}>
                                <FormControlLabel
                                    control={
                                        <Switch
                                            checked={autoApproveShell}
                                            onChange={() => setAutoApproveShell(!autoApproveShell)}
                                            size="medium"
                                        />
                                    }
                                    label={
                                        <Box>
                                            <Typography variant="body1" fontWeight={600}>
                                                Auto-Approve Shell Commands
                                            </Typography>
                                            <Typography variant="body2" color="text.secondary">
                                                Shell commands proposed by the model are executed without manual approval
                                            </Typography>
                                        </Box>
                                    }
                                    sx={{ alignItems: 'flex-start', m: 0 }}
                                />
                            </Box>
                            <Box sx={{
                                p: 2,
                                bgcolor: 'action.hover',
                                borderRadius: 1,
                                border: 1,
                                borderColor: 'divider'
                            }}>
                                <FormControlLabel
                                    control={
                                        <Switch
                                            checked={autoApproveEdits}
                                            onChange={() => setAutoApproveEdits(!autoApproveEdits)}
                                            size="medium"
                                        />
                                    }
                                    label={
                                        <Box>
                                            <Typography variant="body1" fontWeight={600}>
                                                Auto-Approve File Edits
                                            </Typography>
                                            <Typography variant="body2" color="text.secondary">
                                                File edits are applied without manual approval
                                            </Typography>
                                        </Box>
                                    }
                                    sx={{ alignItems: 'flex-start', m: 0 }}
                                />
                            </Box>
                            <Link
                                component="button"
                                underline="hover"
                                onClick={() => OpenProjectDataDir()}
                                sx={{
                                    alignSelf: 'flex-start',
                                    display: 'inline-flex',
                                    alignItems: 'center',
                                    gap: 0.5,
                                    textAlign: 'left'
                                }}
                            >
                                Open project data folder <LaunchIcon fontSize="small" />
                            </Link>
                        </Stack>
                    </Paper>
                )}

                {/* Save Info */}
                <Box sx={{ mt: 4, p: 2, bgcolor: 'info.main', color: 'info.contrastText', borderRadius: 2 }}>
                    <Typography variant="body2">
                        💡 Settings are automatically saved as you make changes
                    </Typography>
                </Box>
            </Box>
        </Box>
    );
}
