import React from 'react';
import { Box, Typography, Accordion, AccordionSummary, AccordionDetails } from '@mui/material';
import ExpandMoreIcon from '@mui/icons-material/ExpandMore';
import MarkdownRenderer from '../../markdown/MarkdownRenderer';
import MarkdownErrorBoundary from '../../markdown/MarkdownErrorBoundary';

type Props = {
    text: string;
    open: boolean;
    onToggle: (open: boolean) => void;
};

function ReasoningPanelComponent({ text, open, onToggle }: Props) {
    if (!text) return null;
    return (
        <Box sx={{ mb: 1 }}>
            <Accordion
                expanded={open}
                onChange={(_, exp) => onToggle(exp)}
                sx={{ boxShadow: 'none', bgcolor: 'transparent', '&:before': { display: 'none' } }}
            >
                <AccordionSummary expandIcon={<ExpandMoreIcon />}>
                    <Typography variant="subtitle2" fontWeight={600}>
                        Planning / Reasoning
                    </Typography>
                </AccordionSummary>
                <AccordionDetails>
                    <MarkdownErrorBoundary>
                        <Box sx={{ color: 'text.secondary', fontSize: '0.9rem' }}>
                            <MarkdownRenderer>{text}</MarkdownRenderer>
                        </Box>
                    </MarkdownErrorBoundary>
                </AccordionDetails>
            </Accordion>
        </Box>
    );
}

export default React.memo(ReasoningPanelComponent);


