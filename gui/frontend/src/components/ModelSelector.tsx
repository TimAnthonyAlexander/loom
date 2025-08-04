import React, { useState, useEffect } from 'react';
import { GetConfig, UpdateModel } from '../../wailsjs/go/main/App';
import './ModelSelector.css';

interface ModelSelectorProps {
    className?: string;
}

const AVAILABLE_MODELS = [
    'claude:claude-opus-4-20250514',
    'claude:claude-sonnet-4-20250514',
    'openai:gpt-4.1',
    'openai:o4-mini',
    'openai:o3',
    'ollama:llama3.1:8b',
    'ollama:qwen3:8b',
    'ollama:gemma3:12b',
    'ollama:mistral:7b',
    'ollama:deepseek-r1:70b',
];

const getModelDisplayName = (model: string): string => {
    // Extract just the model name from the full string
    const parts = model.split(':');
    if (parts.length >= 2) {
        return parts.slice(1).join(':');
    }
    return model;
};

const getProviderIcon = (model: string): string => {
    if (model.startsWith('claude:')) return '🔮';
    if (model.startsWith('openai:')) return '🤖';
    if (model.startsWith('ollama:')) return '🦙';
    return '⚙️';
};

export function ModelSelector({ className }: ModelSelectorProps) {
    const [currentModel, setCurrentModel] = useState<string>('');
    const [isOpen, setIsOpen] = useState<boolean>(false);
    const [isLoading, setIsLoading] = useState<boolean>(false);

    // Load current model on component mount
    useEffect(() => {
        const loadCurrentModel = async () => {
            try {
                const config = await GetConfig();
                if (config && config.model) {
                    setCurrentModel(config.model as string);
                }
            } catch (error) {
                console.error('Failed to load current model:', error);
                // Set a default if loading fails
                setCurrentModel('openai:gpt-4.1');
            }
        };

        loadCurrentModel();
    }, []);

    const handleModelChange = async (newModel: string) => {
        if (newModel === currentModel) {
            setIsOpen(false);
            return;
        }

        setIsLoading(true);
        setIsOpen(false);

        try {
            // Call the backend to update the model
            await UpdateModel(newModel);
            setCurrentModel(newModel);

            // Show success feedback
            console.log(`Model updated to: ${newModel}`);

            // TODO: Add toast notification for success
        } catch (error) {
            console.error('Failed to update model:', error);
            // TODO: Add toast notification for error
        } finally {
            setIsLoading(false);
        }
    };

    const toggleDropdown = () => {
        if (!isLoading) {
            setIsOpen(!isOpen);
        }
    };

    return (
        <div className={`model-selector ${className || ''}`}>
            <div
                className={`model-selector-button ${isOpen ? 'open' : ''} ${isLoading ? 'loading' : ''}`}
                onClick={toggleDropdown}
            >
                <span className="model-icon">
                    {isLoading ? '⏳' : getProviderIcon(currentModel)}
                </span>
                <span className="model-name">
                    {isLoading ? 'Updating...' : getModelDisplayName(currentModel)}
                </span>
                <span className="dropdown-arrow">
                    {isOpen ? '▲' : '▼'}
                </span>
            </div>

            {isOpen && (
                <div className="model-selector-dropdown">
                    {AVAILABLE_MODELS.map((model) => (
                        <div
                            key={model}
                            className={`model-option ${model === currentModel ? 'selected' : ''}`}
                            onClick={() => handleModelChange(model)}
                        >
                            <span className="model-icon">
                                {getProviderIcon(model)}
                            </span>
                            <span className="model-name">
                                {getModelDisplayName(model)}
                            </span>
                            {model === currentModel && (
                                <span className="checkmark">✓</span>
                            )}
                        </div>
                    ))}
                </div>
            )}
        </div>
    );
}
