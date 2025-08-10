import { useState, useEffect } from 'react';
import { Autocomplete, TextField, Chip, Box, Typography } from '@mui/material';

interface ModelOption {
    id: string;
    name: string;
    provider: string;
    group?: string; // Optional group for custom grouping
}

interface ModelSelectorProps {
    onSelect: (model: string) => void;
    currentModel?: string;
}

function ModelSelector({ onSelect, currentModel }: ModelSelectorProps) {
    const [models, setModels] = useState<ModelOption[]>([])
    const [selected, setSelected] = useState<string>(currentModel || '')

    useEffect(() => {
        const availableModels: ModelOption[] = [
            { id: 'openai:gpt-5', name: 'GPT 5', provider: 'openai', group: 'Flagship' },
            { id: 'claude:claude-opus-4-20250514', name: 'Claude Opus 4', provider: 'claude', group: 'Flagship' },
            { id: 'claude:claude-sonnet-4-20250514', name: 'Claude Sonnet 4', provider: 'claude', group: 'Flagship' },

            { id: 'openai:gpt-5', name: 'GPT 5', provider: 'openai', group: 'Reasoning' },
            { id: 'claude:claude-opus-4-20250514', name: 'Claude Opus 4', provider: 'claude', group: 'Reasoning' },
            { id: 'claude:claude-sonnet-4-20250514', name: 'Claude Sonnet 4', provider: 'claude', group: 'Reasoning' },
            { id: 'openai:o4-mini', name: 'o4-mini', provider: 'openai', group: 'Reasoning' },
            { id: 'openai:o3', name: 'o3', provider: 'openai', group: 'Reasoning' },
            { id: 'claude:claude-3-7-sonnet-20250219', name: 'Claude 3.7 Sonnet', provider: 'claude', group: 'Reasoning' },
            { id: 'ollama:deepseek-r1:70b', name: 'DeepSeek R1 (70B)', provider: 'ollama', group: 'Reasoning' },

            { id: 'claude:claude-3-5-haiku-20241022', name: 'Claude 3.5 Haiku', provider: 'claude', group: 'Fast' },
            { id: 'claude:claude-3-haiku-20240307', name: 'Claude 3 Haiku', provider: 'claude', group: 'Fast' },
            { id: 'ollama:llama3:8b', name: 'Llama 3 (8B)', provider: 'ollama', group: 'Fast' },
            { id: 'ollama:mistral:7b', name: 'Mistral (7B)', provider: 'ollama', group: 'Fast' },

            { id: 'ollama:qwen3:8b', name: 'Qwen3 (8B)', provider: 'ollama', group: 'Cheap' },
            { id: 'ollama:gemma3:12b', name: 'Gemma3 (12B)', provider: 'ollama', group: 'Cheap' },
            { id: 'ollama:llama3.1:8b', name: 'Llama 3.1 (8B)', provider: 'ollama', group: 'Cheap' },
            { id: 'ollama:gpt-oss:20b', name: 'GPT-OSS (20B)', provider: 'ollama', group: 'Cheap' },

            { id: 'claude:claude-opus-4-20250514', name: 'Claude Opus 4', provider: 'claude' },
            { id: 'claude:claude-sonnet-4-20250514', name: 'Claude Sonnet 4', provider: 'claude' },
            { id: 'claude:claude-haiku-4-20250514', name: 'Claude Haiku 4', provider: 'claude' },
            { id: 'claude:claude-3-7-sonnet-20250219', name: 'Claude 3.7 Sonnet', provider: 'claude' },
            { id: 'claude:claude-3-5-sonnet-20241022', name: 'Claude 3.5 Sonnet', provider: 'claude' },
            { id: 'claude:claude-3-5-haiku-20241022', name: 'Claude 3.5 Haiku', provider: 'claude' },
            { id: 'claude:claude-3-opus-20240229', name: 'Claude 3 Opus', provider: 'claude' },
            { id: 'claude:claude-3-sonnet-20240229', name: 'Claude 3 Sonnet', provider: 'claude' },
            { id: 'claude:claude-3-haiku-20240307', name: 'Claude 3 Haiku', provider: 'claude' },

            { id: 'openai:gpt-5', name: 'GPT 5', provider: 'openai' },
            { id: 'openai:gpt-4.1', name: 'GPT-4.1', provider: 'openai' },
            { id: 'openai:o4-mini', name: 'o4-mini', provider: 'openai' },
            { id: 'openai:o3', name: 'o3', provider: 'openai' },
            { id: 'openai:o3-mini', name: 'o3', provider: 'openai' },

            { id: 'ollama:llama3.1:8b', name: 'Llama 3.1 (8B)', provider: 'ollama' },
            { id: 'ollama:llama3:8b', name: 'Llama 3 (8B)', provider: 'ollama' },
            { id: 'ollama:gpt-oss:20b', name: 'GPT-OSS (20B)', provider: 'ollama' },
            { id: 'ollama:qwen3:8b', name: 'Qwen3 (8B)', provider: 'ollama' },
            { id: 'ollama:gemma3:12b', name: 'Gemma3 (12B)', provider: 'ollama' },
            { id: 'ollama:mistral:7b', name: 'Mistral (7B)', provider: 'ollama' },
            { id: 'ollama:deepseek-r1:70b', name: 'DeepSeek R1 (70B)', provider: 'ollama' }
        ]

        setModels(availableModels)
        // Do not auto-select a default; wait for persisted currentModel or user selection
        // If parent passes a currentModel later, it will be applied in the next effect
    }, [currentModel, onSelect])

    useEffect(() => {
        if (currentModel) setSelected(currentModel)
    }, [currentModel])

    const value = models.find((m) => m.id === selected) || null

    return (
        <Autocomplete
            size="small"
            options={models}
            value={value}
            onChange={(_, option) => {
                const id = option?.id || ''
                setSelected(id)
                if (id) onSelect(id)
            }}
            groupBy={(option) => option.group ?? option.provider}
            getOptionLabel={(option) => option.name}
            renderOption={(props, option) => (
                <Box component="li" {...props} key={option.id} sx={{ display: 'flex', gap: 1, alignItems: 'center' }}>
                    <Typography variant="body2" sx={{ flex: 1 }}>
                        {option.name}
                    </Typography>
                    <Chip size="small" label={option.provider} variant="outlined" />
                </Box>
            )}
            renderInput={(params) => (
                <TextField {...params} label="Model" placeholder="Select a model" sx={{ minWidth: 180 }} />
            )}
            isOptionEqualToValue={(opt, val) => opt.id === val.id}
        />
    )
}

export default ModelSelector;
