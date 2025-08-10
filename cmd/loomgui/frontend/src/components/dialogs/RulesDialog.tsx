import { Dialog, DialogTitle, DialogContent, DialogActions, Button, Stack, Box, Typography, Paper, TextField } from '@mui/material';

type Props = {
    open: boolean;
    userRules: string[];
    setUserRules: (rules: string[]) => void;
    projectRules: string[];
    setProjectRules: (rules: string[]) => void;
    newUserRule: string;
    setNewUserRule: (v: string) => void;
    newProjectRule: string;
    setNewProjectRule: (v: string) => void;
    onSave: () => void;
    onClose: () => void;
};

export default function RulesDialog(props: Props) {
    const { open, userRules, setUserRules, projectRules, setProjectRules, newUserRule, setNewUserRule, newProjectRule, setNewProjectRule, onSave, onClose } = props;

    return (
        <Dialog open={open} onClose={onClose} maxWidth="md" fullWidth>
            <DialogTitle>Rules</DialogTitle>
            <DialogContent dividers>
                <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2} sx={{ mt: 1 }}>
                    <Box sx={{ flex: 1 }}>
                        <Typography variant="subtitle2" sx={{ mb: 1 }}>User Rules (apply to all projects)</Typography>
                        <Paper variant="outlined" sx={{ p: 1 }}>
                            <Stack spacing={1}>
                                {userRules.map((r: string, idx: number) => (
                                    <Stack key={`ur-${idx}`} direction="row" spacing={1} alignItems="center">
                                        <TextField
                                            size="small"
                                            fullWidth
                                            value={r}
                                            onChange={(e) => {
                                                const next = [...userRules];
                                                next[idx] = e.target.value;
                                                setUserRules(next);
                                            }}
                                        />
                                        <Button color="inherit" onClick={() => setUserRules(userRules.filter((_, i) => i !== idx))}>Delete</Button>
                                    </Stack>
                                ))}
                                <Stack direction="row" spacing={1}>
                                    <TextField
                                        size="small"
                                        fullWidth
                                        placeholder="Add a new user rule"
                                        value={newUserRule}
                                        onChange={(e) => setNewUserRule(e.target.value)}
                                        onKeyDown={(e) => {
                                            if ((e as any).key === 'Enter' && newUserRule.trim()) {
                                                setUserRules([...userRules, newUserRule.trim()]);
                                                setNewUserRule('');
                                            }
                                        }}
                                    />
                                    <Button variant="outlined" onClick={() => { if (newUserRule.trim()) { setUserRules([...userRules, newUserRule.trim()]); setNewUserRule(''); } }}>Add</Button>
                                </Stack>
                            </Stack>
                        </Paper>
                    </Box>
                    <Box sx={{ flex: 1 }}>
                        <Typography variant="subtitle2" sx={{ mb: 1 }}>Project Rules (saved in .loom/rules.json)</Typography>
                        <Paper variant="outlined" sx={{ p: 1 }}>
                            <Stack spacing={1}>
                                {projectRules.map((r: string, idx: number) => (
                                    <Stack key={`pr-${idx}`} direction="row" spacing={1} alignItems="center">
                                        <TextField
                                            size="small"
                                            fullWidth
                                            value={r}
                                            onChange={(e) => {
                                                const next = [...projectRules];
                                                next[idx] = e.target.value;
                                                setProjectRules(next);
                                            }}
                                        />
                                        <Button color="inherit" onClick={() => setProjectRules(projectRules.filter((_, i) => i !== idx))}>Delete</Button>
                                    </Stack>
                                ))}
                                <Stack direction="row" spacing={1}>
                                    <TextField
                                        size="small"
                                        fullWidth
                                        placeholder="Add a new project rule"
                                        value={newProjectRule}
                                        onChange={(e) => setNewProjectRule(e.target.value)}
                                        onKeyDown={(e) => {
                                            if ((e as any).key === 'Enter' && newProjectRule.trim()) {
                                                setProjectRules([...projectRules, newProjectRule.trim()]);
                                                setNewProjectRule('');
                                            }
                                        }}
                                    />
                                    <Button variant="outlined" onClick={() => { if (newProjectRule.trim()) { setProjectRules([...projectRules, newProjectRule.trim()]); setNewProjectRule(''); } }}>Add</Button>
                                </Stack>
                            </Stack>
                        </Paper>
                    </Box>
                </Stack>
            </DialogContent>
            <DialogActions>
                <Button onClick={onClose} color="inherit">Close</Button>
                <Button variant="contained" onClick={onSave}>Save</Button>
            </DialogActions>
        </Dialog>
    );
}


