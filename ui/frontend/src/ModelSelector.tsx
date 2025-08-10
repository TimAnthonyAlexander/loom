import { useState, useEffect, Fragment } from 'react';
import {
    Box,
    Chip,
    FormControl,
    InputLabel,
    ListSubheader,
    MenuItem,
    Select,
    SelectChangeEvent,
    Typography,
} from '@mui/material';

interface ModelOption {
    id: string;
    name: string;
    provider: string;
}

interface ModelSelectorProps {
    onSelect: (model: string) => void;
    currentModel?: string;
}

function ModelSelector({ onSelect, currentModel }: ModelSelectorProps) {
    const [models, setModels] = useState<ModelOption[]>([]);
    const [selected, setSelected] = useState<string>(currentModel || '');

    useEffect(() => {
        const availableModels: ModelOption[] = [
            { id: 'openai:gpt-5', name: 'GPT 5', provider: 'openai' },
            { id: 'claude:claude-opus-4-20250514', name: 'Claude Opus 4', provider: 'claude' },
            { id: 'claude:claude-sonnet-4-20250514', name: 'Claude Sonnet 4', provider: 'claude' },
            { id: 'claude:claude-haiku-4-20250514', name: 'Claude Haiku 4', provider: 'claude' },
            { id: 'claude:claude-3-7-sonnet-20250219', name: 'Claude 3.7 Sonnet', provider: 'claude' },
            { id: 'claude:claude-3-5-sonnet-20241022', name: 'Claude 3.5 Sonnet', provider: 'claude' },
            { id: 'claude:claude-3-5-haiku-20241022', name: 'Claude 3.5 Haiku', provider: 'claude' },
            { id: 'claude:claude-3-opus-20240229', name: 'Claude 3 Opus', provider: 'claude' },
            { id: 'claude:claude-3-sonnet-20240229', name: 'Claude 3 Sonnet', provider: 'claude' },
            { id: 'claude:claude-3-haiku-20240307', name: 'Claude 3 Haiku', provider: 'claude' },
            { id: 'openai:gpt-4.1', name: 'GPT-4.1', provider: 'openai' },
            { id: 'openai:o4-mini', name: 'o4-mini', provider: 'openai' },
            { id: 'openai:o3', name: 'o3', provider: 'openai' },
            { id: 'ollama:llama3.1:8b', name: 'Llama 3.1 (8B)', provider: 'ollama' },
            { id: 'ollama:llama3:8b', name: 'Llama 3 (8B)', provider: 'ollama' },
            { id: 'ollama:gpt-oss:20b', name: 'GPT-OSS (20B)', provider: 'ollama' },
            { id: 'ollama:qwen3:8b', name: 'Qwen3 (8B)', provider: 'ollama' },
            { id: 'ollama:gemma3:12b', name: 'Gemma3 (12B)', provider: 'ollama' },
            { id: 'ollama:mistral:7b', name: 'Mistral (7B)', provider: 'ollama' },
            { id: 'ollama:deepseek-r1:70b', name: 'DeepSeek R1 (70B)', provider: 'ollama' },
        ];
        setModels(availableModels);
    }, []);

    useEffect(() => {
        if (currentModel) setSelected(currentModel);
    }, [currentModel]);

    const providersOrder = ['openai', 'claude', 'ollama'];
    const grouped = providersOrder
        .map(p => [p, models.filter(m => m.provider === p)] as const)
        .filter(([_, arr]) => arr.length > 0);

    const handleChange = (e: SelectChangeEvent<string>) => {
        const id = e.target.value;
        setSelected(id);
        if (id) onSelect(id);
    };

    return (
        <FormControl size="small" sx={{ minWidth: 220 }}>
            <InputLabel id="model-select-label">Model</InputLabel>
            <Select
                labelId="model-select-label"
                label="Model"
                value={selected}
                onChange={handleChange}
                renderValue={(val) => {
                    const opt = models.find(m => m.id === val);
                    if (!opt) return '';
                    return (
                        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                            <Typography variant="body2">{opt.name}</Typography>
                            <Chip size="small" label={opt.provider} variant="outlined" />
                        </Box>
                    );
                }}
                MenuProps={{ PaperProps: { sx: { maxHeight: 420 } } }}
                sx={{
                    borderRadius: 2,
                    '& .MuiSelect-select': { display: 'flex', alignItems: 'center', gap: 1 },
                }}
            >
                {grouped.map(([provider, items]) => (
                    <Fragment
                        key={provider}
                    >
                        <ListSubheader
                            sx={{
                                lineHeight: 2.2,
                                fontWeight: 600,
                                textTransform: 'uppercase',
                                borderBottom: '1px solid #e0e0e0',
                                marginBottom: 2,
                                py: 1,
                            }}
                        >
                            {provider}
                        </ListSubheader>
                        {items.map(opt => (
                            <MenuItem key={opt.id} value={opt.id} selected={opt.id === selected}>
                                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, width: '100%' }}>
                                    <Typography variant="body2" sx={{ flex: 1 }}>{opt.name}</Typography>
                                    <Chip size="small" label={opt.provider} variant="outlined" />
                                </Box>
                            </MenuItem>
                        ))}
                    </Fragment>
                ))}
            </Select>
        </FormControl>
    );
}

export default ModelSelector;
