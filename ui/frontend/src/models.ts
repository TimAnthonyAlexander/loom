// Shared model definitions used across the application

export interface ModelOption {
    id: string;
    name: string;
    provider: string;
    group?: string; // Optional group for custom grouping
    pricing?: {
        input?: number; // USD per 1M tokens
        output?: number; // USD per 1M tokens
    };
}

// All available models - this should stay in sync with backend GetAllAvailableModels
export const ALL_AVAILABLE_MODELS: ModelOption[] = [
    // Flagship models
    { id: 'claude:claude-opus-4-20250514', name: 'Claude Opus 4', provider: 'claude', group: 'Flagship' },
    { id: 'claude:claude-sonnet-4-20250514', name: 'Claude Sonnet 4', provider: 'claude', group: 'Flagship' },
    { id: 'openai:gpt-5', name: 'GPT 5', provider: 'openai', group: 'Flagship' },

    // Reasoning models
    { id: 'openai:o3', name: 'o3', provider: 'openai', group: 'Reasoning' },
    { id: 'openai:o4-mini', name: 'o4-mini', provider: 'openai', group: 'Reasoning' },
    { id: 'claude:claude-3-7-sonnet-20250219', name: 'Claude 3.7 Sonnet', provider: 'claude', group: 'Reasoning' },
    { id: 'ollama:deepseek-r1:70b', name: 'DeepSeek R1 (70B)', provider: 'ollama', group: 'Reasoning' },

    // Current Production models
    { id: 'claude:claude-opus-4-20250514', name: 'Claude Opus 4', provider: 'claude', group: 'Flagship' },
    { id: 'claude:claude-sonnet-4-20250514', name: 'Claude Sonnet 4', provider: 'claude', group: 'Flagship' },
    { id: 'claude:claude-3-7-sonnet-20250219', name: 'Claude 3.7 Sonnet', provider: 'claude', group: 'Reasoning' },
    { id: 'claude:claude-3-5-sonnet-20241022', name: 'Claude 3.5 Sonnet', provider: 'claude', group: 'Production' },

    { id: 'openai:gpt-4o', name: 'GPT-4o', provider: 'openai', group: 'Production' },
    { id: 'openai:gpt-4o-mini', name: 'GPT-4o Mini', provider: 'openai', group: 'Production' },
    { id: 'openai:gpt-4.1', name: 'GPT-4.1', provider: 'openai', group: 'Production' },
    { id: 'openai:gpt-4.1-mini', name: 'GPT-4.1-mini', provider: 'openai', group: 'Production' },
    { id: 'openai:gpt-4.1-nano', name: 'GPT-4.1-nano', provider: 'openai', group: 'Production' },

    // Local models (Ollama)
    { id: 'ollama:llama3.1:8b', name: 'Llama 3.1 (8B)', provider: 'ollama', group: 'Local' },
    { id: 'ollama:llama3:8b', name: 'Llama 3 (8B)', provider: 'ollama', group: 'Local' },
    { id: 'ollama:mistral:7b', name: 'Mistral (7B)', provider: 'ollama', group: 'Local' },
    { id: 'ollama:qwen3:8b', name: 'Qwen3 (8B)', provider: 'ollama', group: 'Local' },
    { id: 'ollama:gemma3:12b', name: 'Gemma3 (12B)', provider: 'ollama', group: 'Local' },
    { id: 'ollama:gpt-oss:20b', name: 'GPT-OSS (20B)', provider: 'ollama', group: 'Local' },
];

// Default models that should be selected initially
export const DEFAULT_SELECTED_MODELS = [
    'claude:claude-opus-4-20250514',
    'claude:claude-sonnet-4-20250514',
    'openai:gpt-5',
    'openai:o3',
    'openai:o4-mini',
    'claude:claude-3-7-sonnet-20250219',
    'openrouter:deepseek/deepseek-chat-v3.1:free',
];

// Function to fetch OpenRouter models dynamically
export async function fetchOpenRouterModels(): Promise<ModelOption[]> {
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

// Get all models including dynamic OpenRouter models
export async function getAllModels(): Promise<ModelOption[]> {
    // Start with static models
    let allModels = [...ALL_AVAILABLE_MODELS];

    // Try to fetch dynamic OpenRouter models and merge them
    try {
        const dynamicOpenRouterModels = await fetchOpenRouterModels();
        if (dynamicOpenRouterModels.length > 0) {
            // Remove static OpenRouter models and replace with dynamic ones
            const nonOpenRouterStatic = allModels.filter(m => m.provider !== 'openrouter');
            allModels = [...nonOpenRouterStatic, ...dynamicOpenRouterModels];
        }
    } catch (error) {
        console.warn('Failed to load dynamic OpenRouter models, using static fallback');
    }

    return allModels;
}

// Filter models based on selected model IDs
export function filterSelectedModels(allModels: ModelOption[], selectedModelIds: string[]): ModelOption[] {
    if (!selectedModelIds || selectedModelIds.length === 0) {
        return allModels.filter(model => DEFAULT_SELECTED_MODELS.includes(model.id));
    }

    return allModels.filter(model => selectedModelIds.includes(model.id));
}
