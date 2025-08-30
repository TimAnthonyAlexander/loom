import React, { useState } from 'react';
import {
    Dialog,
    DialogTitle,
    DialogContent,
    DialogActions,
    Button,
    Stack,
    TextField,
    Typography,
    FormControl,
    InputLabel,
    Select,
    MenuItem,
    FormControlLabel,
    Checkbox,
    Chip,
    Box,
} from '@mui/material';
import FolderIcon from '@mui/icons-material/Folder';

type Props = {
    open: boolean;
    onClose: () => void;
    onCreate: (config: NewProjectConfig) => void;
    onBrowsePath: () => Promise<string>;
};

export type NewProjectConfig = {
    name: string;
    path: string;
    language: string;
    frameworks: string;
    initGit: boolean;
};

const LANGUAGES = [
    { value: '', label: 'None' },
    { value: 'javascript', label: 'JavaScript' },
    { value: 'typescript', label: 'TypeScript' },
    { value: 'python', label: 'Python' },
    { value: 'go', label: 'Go' },
    { value: 'rust', label: 'Rust' },
    { value: 'java', label: 'Java' },
    { value: 'cpp', label: 'C++' },
    { value: 'csharp', label: 'C#' },
];

const FRAMEWORK_OPTIONS = {
    javascript: ['React', 'Vue', 'Angular', 'Express', 'Next.js', 'Nuxt.js', 'Svelte'],
    typescript: ['React', 'Vue', 'Angular', 'Express', 'Next.js', 'Nuxt.js', 'Svelte', 'NestJS'],
    python: ['Django', 'Flask', 'FastAPI', 'Tornado', 'Pyramid'],
    go: ['Gin', 'Echo', 'Fiber', 'Gorilla/Mux'],
    rust: ['Actix-web', 'Warp', 'Rocket', 'Axum'],
    java: ['Spring Boot', 'Spring MVC', 'Quarkus', 'Micronaut'],
    cpp: ['Qt', 'FLTK', 'wxWidgets'],
    csharp: ['.NET Core', 'ASP.NET', 'Blazor', 'MAUI'],
};

export default function NewProjectDialog({ open, onClose, onCreate, onBrowsePath }: Props) {
    const [name, setName] = useState('');
    const [basePath, setBasePath] = useState('');
    const [language, setLanguage] = useState('');
    const [frameworks, setFrameworks] = useState<string[]>([]);
    const [frameworkInput, setFrameworkInput] = useState('');
    const [initGit, setInitGit] = useState(true);

    const handleClose = () => {
        setName('');
        setBasePath('');
        setLanguage('');
        setFrameworks([]);
        setFrameworkInput('');
        setInitGit(true);
        onClose();
    };

    const handleCreate = () => {
        if (!name.trim() || !basePath.trim()) {
            return;
        }

        const config: NewProjectConfig = {
            name: name.trim(),
            path: basePath.trim(),
            language,
            frameworks: frameworks.join(', '),
            initGit,
        };

        onCreate(config);
        handleClose();
    };

    const handleBrowse = async () => {
        try {
            const path = await onBrowsePath();
            if (path) {
                setBasePath(path);
            }
        } catch (error) {
            console.error('Failed to browse path:', error);
        }
    };

    const handleLanguageChange = (newLanguage: string) => {
        setLanguage(newLanguage);
        setFrameworks([]); // Clear frameworks when language changes
    };

    const handleAddFramework = () => {
        const trimmed = frameworkInput.trim();
        if (trimmed && !frameworks.includes(trimmed)) {
            setFrameworks([...frameworks, trimmed]);
            setFrameworkInput('');
        }
    };

    const handleRemoveFramework = (framework: string) => {
        setFrameworks(frameworks.filter(f => f !== framework));
    };

    const handleFrameworkKeyDown = (event: React.KeyboardEvent) => {
        if (event.key === 'Enter' || event.key === ',') {
            event.preventDefault();
            handleAddFramework();
        }
    };

    const availableFrameworks = language ? FRAMEWORK_OPTIONS[language as keyof typeof FRAMEWORK_OPTIONS] || [] : [];

    return (
        <Dialog open={open} onClose={handleClose} maxWidth="md" fullWidth>
            <DialogTitle sx={{
                background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
                color: 'white',
                borderRadius: '4px 4px 0 0'
            }}>
                Create New Project
            </DialogTitle>
            <DialogContent dividers sx={{ pb: 3 }}>
                <Stack spacing={3} sx={{ mt: 2 }}>
                    {/* Project Name */}
                    <TextField
                        label="Project Name"
                        value={name}
                        onChange={(e) => setName(e.target.value)}
                        placeholder="my-awesome-project"
                        fullWidth
                        required
                        variant="outlined"
                        helperText="This will be the folder name and default package name"
                    />

                    {/* Base Path */}
                    <Stack direction="row" spacing={1}>
                        <TextField
                            label="Location"
                            value={basePath}
                            onChange={(e) => setBasePath(e.target.value)}
                            placeholder="/path/to/projects"
                            fullWidth
                            required
                            variant="outlined"
                        />
                        <Button
                            variant="outlined"
                            onClick={handleBrowse}
                            sx={{
                                minWidth: 'auto',
                                px: 2,
                                borderColor: '#667eea',
                                color: '#667eea',
                                '&:hover': {
                                    borderColor: '#764ba2',
                                    color: '#764ba2',
                                    backgroundColor: 'rgba(102, 126, 234, 0.04)'
                                }
                            }}
                        >
                            <FolderIcon />
                        </Button>
                    </Stack>

                    {/* Full Path Preview */}
                    {name && basePath && (
                        <Box sx={{
                            p: 2,
                            borderRadius: 1,
                            border: '1px solid',
                            borderColor: 'grey.200'
                        }}>
                            <Typography variant="caption" color="text.secondary">
                                Project will be created at:
                            </Typography>
                            <Typography variant="body2" sx={{ fontFamily: 'monospace', mt: 0.5 }}>
                                {basePath}/{name}
                            </Typography>
                        </Box>
                    )}

                    {/* Language Selection */}
                    <FormControl fullWidth>
                        <InputLabel>Language</InputLabel>
                        <Select
                            value={language}
                            label="Language"
                            onChange={(e) => handleLanguageChange(e.target.value)}
                        >
                            {LANGUAGES.map((lang) => (
                                <MenuItem key={lang.value} value={lang.value}>
                                    {lang.label}
                                </MenuItem>
                            ))}
                        </Select>
                    </FormControl>

                    {/* Frameworks */}
                    {language && (
                        <Box>
                            <Stack direction="row" spacing={1} sx={{ mb: 1 }}>
                                <TextField
                                    label="Add Framework"
                                    value={frameworkInput}
                                    onChange={(e) => setFrameworkInput(e.target.value)}
                                    onKeyDown={handleFrameworkKeyDown}
                                    onBlur={handleAddFramework}
                                    placeholder="Type or select from suggestions"
                                    size="small"
                                    sx={{ flexGrow: 1 }}
                                />
                                <Button
                                    onClick={handleAddFramework}
                                    disabled={!frameworkInput.trim()}
                                    variant="outlined"
                                    size="small"
                                >
                                    Add
                                </Button>
                            </Stack>

                            {/* Framework Suggestions */}
                            {availableFrameworks.length > 0 && (
                                <Box sx={{ mb: 2 }}>
                                    <Typography variant="caption" color="text.secondary" sx={{ mb: 1, display: 'block' }}>
                                        Popular for {LANGUAGES.find(l => l.value === language)?.label}:
                                    </Typography>
                                    <Stack direction="row" spacing={0.5} flexWrap="wrap" useFlexGap>
                                        {availableFrameworks
                                            .filter(fw => !frameworks.includes(fw))
                                            .map((fw) => (
                                                <Chip
                                                    key={fw}
                                                    label={fw}
                                                    size="small"
                                                    clickable
                                                    variant="outlined"
                                                    onClick={() => setFrameworks([...frameworks, fw])}
                                                    sx={{
                                                        '&:hover': {
                                                            backgroundColor: 'primary.main',
                                                            color: 'white'
                                                        }
                                                    }}
                                                />
                                            ))
                                        }
                                    </Stack>
                                </Box>
                            )}

                            {/* Selected Frameworks */}
                            {frameworks.length > 0 && (
                                <Box>
                                    <Typography variant="caption" color="text.secondary" sx={{ mb: 1, display: 'block' }}>
                                        Selected frameworks:
                                    </Typography>
                                    <Stack direction="row" spacing={0.5} flexWrap="wrap" useFlexGap>
                                        {frameworks.map((fw) => (
                                            <Chip
                                                key={fw}
                                                label={fw}
                                                size="small"
                                                onDelete={() => handleRemoveFramework(fw)}
                                                color="primary"
                                            />
                                        ))}
                                    </Stack>
                                </Box>
                            )}
                        </Box>
                    )}

                    {/* Git Initialization */}
                    <FormControlLabel
                        control={
                            <Checkbox
                                checked={initGit}
                                onChange={(e) => setInitGit(e.target.checked)}
                                sx={{
                                    color: '#667eea',
                                    '&.Mui-checked': {
                                        color: '#667eea',
                                    },
                                }}
                            />
                        }
                        label={
                            <Box>
                                <Typography variant="body2">Initialize Git repository</Typography>
                                <Typography variant="caption" color="text.secondary">
                                    Creates .git folder and basic .gitignore file
                                </Typography>
                            </Box>
                        }
                    />
                </Stack>
            </DialogContent>
            <DialogActions sx={{ p: 2, gap: 1 }}>
                <Button onClick={handleClose} color="inherit">
                    Cancel
                </Button>
                <Button
                    variant="contained"
                    onClick={handleCreate}
                    disabled={!name.trim() || !basePath.trim()}
                    sx={{
                        background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
                        '&:hover': {
                            background: 'linear-gradient(135deg, #5a6fd8 0%, #6a4190 100%)',
                        },
                        '&:disabled': {
                            background: 'grey.300',
                        }
                    }}
                >
                    Create Project
                </Button>
            </DialogActions>
        </Dialog>
    );
}
