import React from 'react';
import { Dialog, DialogTitle, DialogContent, DialogActions, Button, Box, Typography, Divider } from '@mui/material';

export type CostBucket = {
    provider: string; // "openai" | "anthropic"
    model: string;    // model id
    inTokens: number;
    outTokens: number;
    inUSD: number;
    outUSD: number;
    totalUSD: number;
};

type CostsDialogProps = {
    open: boolean;
    onClose: () => void;
    // This project
    totalInUSD: number;
    totalOutUSD: number;
    totalInTokens: number;
    totalOutTokens: number;
    perProvider: Record<string, { inUSD: number; outUSD: number; totalUSD: number; inTokens: number; outTokens: number; totalTokens: number }>;
    perModel: Record<string, { provider: string; inUSD: number; outUSD: number; totalUSD: number }>;
    // All projects (global)
    gTotalInUSD: number;
    gTotalOutUSD: number;
    gTotalInTokens: number;
    gTotalOutTokens: number;
    gPerProvider: Record<string, { inUSD: number; outUSD: number; totalUSD: number; inTokens: number; outTokens: number; totalTokens: number }>;
    gPerModel: Record<string, { provider: string; inUSD: number; outUSD: number; totalUSD: number }>;
};

const currency = (v: number) => `$${(v || 0).toFixed(2)}`;

const CostsDialog: React.FC<CostsDialogProps> = ({ open, onClose, totalInUSD, totalOutUSD, totalInTokens, totalOutTokens, perProvider, perModel, gTotalInUSD, gTotalOutUSD, gTotalInTokens, gTotalOutTokens, gPerProvider, gPerModel }) => {
    const totalUSD = (totalInUSD || 0) + (totalOutUSD || 0);
    const totalTokens = (totalInTokens || 0) + (totalOutTokens || 0);
    const gTotalUSD = (gTotalInUSD || 0) + (gTotalOutUSD || 0);
    const gTotalTokens = (gTotalInTokens || 0) + (gTotalOutTokens || 0);

    return (
        <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
            <DialogTitle>Usage & Costs</DialogTitle>
            <DialogContent>
                {/* Global (all projects) */}
                <Typography variant="subtitle1" fontWeight={600} gutterBottom>All projects</Typography>
                <Box display="flex" justifyContent="space-between" mb={1}>
                    <Typography variant="subtitle1">Total</Typography>
                    <Typography variant="subtitle1" fontWeight={600}>{currency(gTotalUSD)}</Typography>
                </Box>
                <Box display="flex" justifyContent="space-between" sx={{ color: 'text.secondary' }}>
                    <Typography>In</Typography>
                    <Typography>{currency(gTotalInUSD)}</Typography>
                </Box>
                <Box display="flex" justifyContent="space-between" sx={{ color: 'text.secondary' }}>
                    <Typography>Out</Typography>
                    <Typography>{currency(gTotalOutUSD)}</Typography>
                </Box>
                <Divider sx={{ my: 2 }} />
                <Typography variant="subtitle2" gutterBottom>Tokens</Typography>
                <Box display="flex" justifyContent="space-between" sx={{ color: 'text.secondary' }}>
                    <Typography>In</Typography>
                    <Typography>{(gTotalInTokens || 0).toLocaleString()}</Typography>
                </Box>
                <Box display="flex" justifyContent="space-between" sx={{ color: 'text.secondary' }}>
                    <Typography>Out</Typography>
                    <Typography>{(gTotalOutTokens || 0).toLocaleString()}</Typography>
                </Box>
                <Box display="flex" justifyContent="space-between">
                    <Typography variant="subtitle1">Total</Typography>
                    <Typography variant="subtitle1" fontWeight={600}>{gTotalTokens}</Typography>
                </Box>
                <Divider sx={{ my: 2 }} />
                <Typography variant="subtitle2" gutterBottom>By Provider</Typography>
                {Object.entries(gPerProvider || {}).map(([prov, v]) => (
                    <Box key={prov} sx={{ mb: 1 }}>
                        <Box display="flex" justifyContent="space-between">
                            <Typography sx={{ textTransform: 'capitalize' }}>{prov}</Typography>
                            <Typography>{currency(v.totalUSD || 0)}</Typography>
                        </Box>
                        <Box display="flex" justifyContent="space-between" sx={{ color: 'text.secondary' }}>
                            <Typography variant="caption">Tokens In/Out</Typography>
                            <Typography variant="caption">{(v.inTokens || 0).toLocaleString()} / {(v.outTokens || 0).toLocaleString()}</Typography>
                        </Box>
                    </Box>
                ))}
                <Divider sx={{ my: 2 }} />
                <Typography variant="subtitle2" gutterBottom>By Model</Typography>
                {Object.entries(gPerModel || {}).map(([model, v]) => (
                    <Box key={model} display="flex" justifyContent="space-between" sx={{ mb: 0.5 }}>
                        <Typography>{v.provider}:{model}</Typography>
                        <Typography>{currency(v.totalUSD || 0)}</Typography>
                    </Box>
                ))}

                {/* This project */}
                <Divider sx={{ my: 3 }} />
                <Typography variant="subtitle1" fontWeight={600} gutterBottom>This project</Typography>
                <Box display="flex" justifyContent="space-between" sx={{ color: 'text.secondary' }}>
                    <Typography>In</Typography>
                    <Typography>{currency(totalInUSD)}</Typography>
                </Box>
                <Box display="flex" justifyContent="space-between" sx={{ color: 'text.secondary' }}>
                    <Typography>Out</Typography>
                    <Typography>{currency(totalOutUSD)}</Typography>
                </Box>
                <Box display="flex" justifyContent="space-between">
                    <Typography variant="subtitle1">Total</Typography>
                    <Typography variant="subtitle1" fontWeight={600}>{currency(totalUSD)}</Typography>
                </Box>
            </DialogContent>
            <DialogActions>
                <Button onClick={onClose} variant="contained">Close</Button>
            </DialogActions>
        </Dialog>
    );
};

export default CostsDialog;


