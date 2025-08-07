import React, { useState, useEffect } from 'react';
import './ModelSelector.css';

interface ModelOption {
    id: string;
    name: string;
    provider: string;
}

interface ModelSelectorProps {
    onSelect: (model: string) => void;
    currentModel?: string;
}

const ModelSelector: React.FC<ModelSelectorProps> = ({ onSelect, currentModel }) => {
    const [models, setModels] = useState<ModelOption[]>([]);
    const [selected, setSelected] = useState<string>(currentModel || '');

    useEffect(() => {
        // These would ideally come from the backend
        const availableModels: ModelOption[] = [
            { id: 'openai:gpt-5', name: 'GPT 5', provider: 'openai' },
            // Claude 4 Models
            { id: 'claude:claude-opus-4-20250514', name: 'Claude Opus 4', provider: 'claude' },
            { id: 'claude:claude-sonnet-4-20250514', name: 'Claude Sonnet 4', provider: 'claude' },
            { id: 'claude:claude-haiku-4-20250514', name: 'Claude Haiku 4', provider: 'claude' },

            // Claude 3.7 Models
            { id: 'claude:claude-3-7-sonnet-20250219', name: 'Claude 3.7 Sonnet', provider: 'claude' },

            // Claude 3.5 Models
            { id: 'claude:claude-3-5-sonnet-20241022', name: 'Claude 3.5 Sonnet', provider: 'claude' },
            { id: 'claude:claude-3-5-haiku-20241022', name: 'Claude 3.5 Haiku', provider: 'claude' },

            // Claude 3 Models
            { id: 'claude:claude-3-opus-20240229', name: 'Claude 3 Opus', provider: 'claude' },
            { id: 'claude:claude-3-sonnet-20240229', name: 'Claude 3 Sonnet', provider: 'claude' },
            { id: 'claude:claude-3-haiku-20240307', name: 'Claude 3 Haiku', provider: 'claude' },

            // OpenAI Models
            { id: 'openai:gpt-4.1', name: 'GPT-4.1', provider: 'openai' },
            { id: 'openai:o4-mini', name: 'o4-mini', provider: 'openai' },
            { id: 'openai:o3', name: 'o3', provider: 'openai' },

            // Ollama Models
            { id: 'ollama:llama3.1:8b', name: 'Llama 3.1 (8B)', provider: 'ollama' },
            { id: 'ollama:llama3:8b', name: 'Llama 3 (8B)', provider: 'ollama' },
            { id: 'ollama:gpt-oss:20b', name: 'GPT-OSS (20B)', provider: 'ollama' },
            { id: 'ollama:qwen3:8b', name: 'Qwen3 (8B)', provider: 'ollama' },
            { id: 'ollama:gemma3:12b', name: 'Gemma3 (12B)', provider: 'ollama' },
            { id: 'ollama:mistral:7b', name: 'Mistral (7B)', provider: 'ollama' },
            { id: 'ollama:deepseek-r1:70b', name: 'DeepSeek R1 (70B)', provider: 'ollama' },
        ];

        setModels(availableModels);

        // If there's no current model set, default to the first one
        if (!currentModel && availableModels.length > 0) {
            setSelected(availableModels[0].id);
            onSelect(availableModels[0].id);
        }
    }, [currentModel, onSelect]);

    const handleChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
        const value = e.target.value;
        setSelected(value);
        onSelect(value);
    };

    return (
        <div className="model-selector">
            <select value={selected} onChange={handleChange}>
                {models.map((model) => (
                    <option key={model.id} value={model.id}>
                        {model.name} ({model.provider})
                    </option>
                ))}
            </select>
        </div>
    );
};

export default ModelSelector;
