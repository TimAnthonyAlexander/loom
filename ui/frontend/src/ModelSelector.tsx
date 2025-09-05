import React, { useState, useEffect } from 'react';
import { Autocomplete, TextField, Box, Typography } from '@mui/material';
import { ModelOption, getAllModels, filterSelectedModels } from './models';

interface ModelSelectorProps {
    onSelect: (model: string) => void;
    currentModel?: string;
    selectedModels?: string[];
}

const ModelSelector = React.memo(function ModelSelector({ onSelect, currentModel, selectedModels }: ModelSelectorProps) {
    const [models, setModels] = useState<ModelOption[]>([])
    const [selected, setSelected] = useState<string>(currentModel || '')

    useEffect(() => {
        async function loadModels() {
            try {
                // Get all available models
                const allModels = await getAllModels();
                // Filter based on selected models from settings
                const filteredModels = filterSelectedModels(allModels, selectedModels || []);
                setModels(filteredModels);
            } catch (error) {
                console.warn('Failed to load models:', error);
            }
        }

        loadModels();
    }, [selectedModels])

    useEffect(() => {
        if (currentModel) setSelected(currentModel)
    }, [currentModel])

    // @ts-ignore
    const value = models.find((m) => m.id === selected) || null;

    return (
        <Autocomplete
            size="small"
            options={models}
            value={value!}
            onChange={(_, option) => {
                const id = option?.id || '';
                setSelected(id);
                if (id) onSelect(id);
            }}
            groupBy={(option) => option.group ?? option.provider}
            getOptionLabel={(option) => option.name}
            disableClearable
            forcePopupIcon={false}
            renderOption={(props, option) => (
                <Box
                    component="li"
                    {...props}
                    key={option.id}
                    sx={{
                        display: 'flex',
                        flexDirection: 'column',
                        alignItems: 'flex-start',
                        justifyContent: 'flex-start', // makes sure it's not centered vertically
                        textAlign: 'left',            // forces left alignment for text
                        width: '100%',
                    }}
                >
                    <Typography
                        variant="body2"
                        sx={{ fontWeight: 500, textAlign: 'left', width: '100%' }}
                    >
                        {option.name}
                    </Typography>
                    {!option.pricing && option.provider &&
                        <Typography
                            variant="caption"
                            sx={{
                                color: 'text.secondary',
                                fontSize: '0.65rem',
                                textAlign: 'left',
                                width: '100%',
                                mt: 0.25,
                            }}
                        >
                            Provider: {option.provider}
                        </Typography>
                    }
                    {option.pricing && (
                        <Typography
                            variant="caption"
                            sx={{
                                color: 'text.secondary',
                                fontSize: '0.65rem',
                                textAlign: 'left',
                                width: '100%',
                                mt: 0.25,
                            }}
                        >
                            ${option.pricing.input?.toFixed(2)}/M in • $
                            {option.pricing.output?.toFixed(2)}/M out
                        </Typography>
                    )}
                </Box>
            )}
            renderInput={(params) => (
                <TextField
                    {...params}
                    placeholder="Select a model"
                    sx={{
                        width: '100%',
                        minWidth: 180,
                        '& .MuiOutlinedInput-root': {
                            '& fieldset': {
                                border: 'none',
                            },
                            '&:hover fieldset': {
                                border: 'none',
                            },
                            '&.Mui-focused fieldset': {
                                border: 'none',
                            },
                        },
                    }}
                />
            )}
            isOptionEqualToValue={(opt, val) => opt.id === val.id}
        />
    )
});

export default ModelSelector;
