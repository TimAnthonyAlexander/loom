import React, { useState, useEffect } from 'react';
import { Autocomplete, TextField, Chip, Box, Typography } from '@mui/material';

interface ModelOption {
    id: string;
    name: string;
    provider: string;
    group?: string; // Optional group for custom grouping
    pricing?: {
        input?: number; // USD per 1M tokens
        output?: number; // USD per 1M tokens
    };
}

interface ModelSelectorProps {
    onSelect: (model: string) => void;
    currentModel?: string;
}

// Function to fetch OpenRouter models with pricing
async function fetchOpenRouterModels(): Promise<ModelOption[]> {
    try {
        const response = await fetch('https://openrouter.ai/api/v1/models');
        if (!response.ok) {
            console.warn('Failed to fetch OpenRouter models:', response.statusText);
            return [];
        }

        const data = await response.json();
        const models: ModelOption[] = [];

        for (const model of data.data || []) {
            if (!model.id || !model.name) continue;

            // Convert pricing from per-token to per-million tokens
            let pricing: { input?: number; output?: number } | undefined;
            if (model.pricing?.prompt && model.pricing?.completion) {
                pricing = {
                    input: parseFloat(model.pricing.prompt) * 1_000_000,
                    output: parseFloat(model.pricing.completion) * 1_000_000,
                };
            }

            models.push({
                id: `openrouter:${model.id}`,
                name: model.name,
                provider: 'openrouter',
                group: 'OpenRouter',
                pricing,
            });
        }

        // Sort by input pricing (cheapest first)
        return models.sort((a, b) => {
            const aPrice = a.pricing?.input || Infinity;
            const bPrice = b.pricing?.input || Infinity;
            return aPrice - bPrice;
        });
    } catch (error) {
        console.warn('Error fetching OpenRouter models:', error);
        return [];
    }
}

const ModelSelector = React.memo(function ModelSelector({ onSelect, currentModel }: ModelSelectorProps) {
    const [models, setModels] = useState<ModelOption[]>([])
    const [selected, setSelected] = useState<string>(currentModel || '')

    useEffect(() => {
        async function loadModels() {
            const staticModels: ModelOption[] = [
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

                { id: 'openai:gpt-4.1-mini', name: 'GPT-4.1-mini', provider: 'openai', group: 'Cheap' },
                { id: 'openai:gpt-4.1-nano', name: 'GPT-4.1-nano', provider: 'openai', group: 'Cheap' },
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
                { id: 'openai:gpt-4.1-mini', name: 'GPT-4.1-mini', provider: 'openai', group: 'openai' },
                { id: 'openai:gpt-4.1-nano', name: 'GPT-4.1-nano', provider: 'openai', group: 'openai' },

                { id: 'ollama:llama3.1:8b', name: 'Llama 3.1 (8B)', provider: 'ollama' },
                { id: 'ollama:llama3:8b', name: 'Llama 3 (8B)', provider: 'ollama' },
                { id: 'ollama:gpt-oss:20b', name: 'GPT-OSS (20B)', provider: 'ollama' },
                { id: 'ollama:qwen3:8b', name: 'Qwen3 (8B)', provider: 'ollama' },
                { id: 'ollama:gemma3:12b', name: 'Gemma3 (12B)', provider: 'ollama' },
                { id: 'ollama:mistral:7b', name: 'Mistral (7B)', provider: 'ollama' },
                { id: 'ollama:deepseek-r1:70b', name: 'DeepSeek R1 (70B)', provider: 'ollama' },

                // OpenRouter models - popular ones with manual pricing for now
                { id: 'openrouter:anthropic/claude-3.5-sonnet', name: 'Claude 3.5 Sonnet', provider: 'openrouter', group: 'OpenRouter', pricing: { input: 3, output: 15 } },
                { id: 'openrouter:openai/gpt-4o', name: 'GPT-4o', provider: 'openrouter', group: 'OpenRouter', pricing: { input: 5, output: 15 } },
                { id: 'openrouter:openai/gpt-4o-mini', name: 'GPT-4o Mini', provider: 'openrouter', group: 'OpenRouter', pricing: { input: 0.15, output: 0.6 } },
                { id: 'openrouter:anthropic/claude-3-haiku', name: 'Claude 3 Haiku', provider: 'openrouter', group: 'OpenRouter', pricing: { input: 0.25, output: 1.25 } },
                { id: 'openrouter:meta-llama/llama-3.1-70b-instruct', name: 'Llama 3.1 70B', provider: 'openrouter', group: 'OpenRouter', pricing: { input: 0.9, output: 0.9 } },
                { id: 'openrouter:deepseek/deepseek-chat', name: 'DeepSeek Chat', provider: 'openrouter', group: 'OpenRouter', pricing: { input: 0.14, output: 0.28 } },
                { id: 'openrouter:google/gemini-pro', name: 'Gemini Pro', provider: 'openrouter', group: 'OpenRouter', pricing: { input: 1.25, output: 5 } },
                { id: 'openrouter:mistralai/mistral-large', name: 'Mistral Large', provider: 'openrouter', group: 'OpenRouter', pricing: { input: 3, output: 9 } }
            ]

            // Start with static models
            setModels(staticModels);

            // Try to fetch dynamic OpenRouter models and merge them
            try {
                const dynamicOpenRouterModels = await fetchOpenRouterModels();
                if (dynamicOpenRouterModels.length > 0) {
                    // Remove static OpenRouter models and replace with dynamic ones
                    const nonOpenRouterStatic = staticModels.filter(m => m.provider !== 'openrouter');
                    const allModels = [...nonOpenRouterStatic, ...dynamicOpenRouterModels];
                    setModels(allModels);
                }
            } catch (error) {
                console.warn('Failed to load dynamic OpenRouter models, using static fallback');
            }
        }

        loadModels();
        // Only run this effect once on mount - models don't need to reload based on props
        // The onSelect callback doesn't affect which models are available
    }, [])

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
                    <Box sx={{ flex: 1 }}>
                        <Typography variant="body2">
                            {option.name}
                        </Typography>
                        {option.pricing && (
                            <Typography variant="caption" sx={{ color: 'text.secondary' }}>
                                ${option.pricing.input?.toFixed(2)}/M in • ${option.pricing.output?.toFixed(2)}/M out
                            </Typography>
                        )}
                    </Box>
                    <Chip size="small" label={option.provider} variant="outlined" />
                </Box>
            )}
            renderInput={(params) => (
                <TextField {...params} label="Model" placeholder="Select a model" sx={{ minWidth: 350, maxWidth: 500 }} />
            )}
            isOptionEqualToValue={(opt, val) => opt.id === val.id}
        />
    )
});

export default ModelSelector;
