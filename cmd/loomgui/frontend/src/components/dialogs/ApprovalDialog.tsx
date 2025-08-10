import { Dialog, DialogTitle, DialogContent, DialogActions, Button, Typography, Box } from '@mui/material';
import DiffViewer from '../diff/DiffViewer';

type Props = {
    open: boolean;
    summary?: string;
    diff?: string;
    onApprove: () => void;
    onReject: () => void;
    onClose: () => void;
};

export default function ApprovalDialog({ open, summary, diff, onApprove, onReject, onClose }: Props) {
    return (
        <Dialog open={open} onClose={onClose} maxWidth="md" fullWidth>
            <DialogTitle>Action Requires Approval</DialogTitle>
            <DialogContent dividers>
                {summary && (
                    <Typography variant="subtitle1" sx={{ mb: 2 }}>
                        {summary}
                    </Typography>
                )}
                <Box sx={{
                    bgcolor: 'background.paper',
                    p: 2,
                    borderRadius: 1,
                    border: '1px solid',
                    borderColor: 'divider',
                    overflow: 'auto',
                    maxHeight: 400,
                    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
                }}>
                    <DiffViewer diff={diff} />
                </Box>
            </DialogContent>
            <DialogActions>
                <Button color="inherit" onClick={onReject}>Reject</Button>
                <Button variant="contained" onClick={onApprove}>Approve</Button>
            </DialogActions>
        </Dialog>
    );
}


