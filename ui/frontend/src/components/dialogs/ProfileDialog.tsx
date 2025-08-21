import React from 'react';
import {
    Dialog,
    DialogTitle,
    DialogContent,
    DialogActions,
    Button,
    Box,
    Typography,
    Table,
    TableBody,
    TableCell,
    TableContainer,
    TableHead,
    TableRow,
    Paper,
    Chip,
    Pagination,
    TextField,
    Tab,
    Tabs,
    Accordion,
    AccordionSummary,
    AccordionDetails,
    List,
    ListItem,
    ListItemText,
    CircularProgress,
} from '@mui/material';
import ExpandMoreIcon from '@mui/icons-material/ExpandMore';

type Symbol = {
    id?: string;
    name?: string;
    kind?: string;
    file?: string;
    line?: number;
    doc?: string;
    lang?: string;
};

type ProfileData = {
    symbols: {
        symbols: Symbol[];
        total: number;
        page: number;
        limit: number;
    };
    hotlist: string[];
    rules: string;
    profile: any;
};

type ProfileDialogProps = {
    open: boolean;
    onClose: () => void;
};

function TabPanel(props: { children?: React.ReactNode; value: number; index: number }) {
    const { children, value, index } = props;
    return (
        <div role="tabpanel" hidden={value !== index}>
            {value === index && <Box sx={{ pt: 2 }}>{children}</Box>}
        </div>
    );
}

const ProfileDialog: React.FC<ProfileDialogProps> = ({ open, onClose }) => {
    const [profileData, setProfileData] = React.useState<ProfileData | null>(null);
    const [loading, setLoading] = React.useState(false);
    const [tabValue, setTabValue] = React.useState(0);
    const [symbolsPage, setSymbolsPage] = React.useState(0);
    const [symbolsSearch, setSymbolsSearch] = React.useState('');
    const symbolsLimit = 20;

    const fetchProfileData = React.useCallback(async (page = 0) => {
        setLoading(true);
        try {
            const anyWin: any = window as any;
            const data = await anyWin?.go?.bridge?.App?.GetProfileData?.(page, symbolsLimit);
            if (data) {
                setProfileData(data);
            }
        } catch (error) {
            console.error('Failed to fetch profile data:', error);
        } finally {
            setLoading(false);
        }
    }, []);

    React.useEffect(() => {
        if (open) {
            fetchProfileData(symbolsPage);
        }
    }, [open, symbolsPage, fetchProfileData]);

    const handleTabChange = (_event: React.SyntheticEvent, newValue: number) => {
        setTabValue(newValue);
    };

    const handleSymbolsPageChange = (_event: React.ChangeEvent<unknown>, page: number) => {
        setSymbolsPage(page - 1); // MUI Pagination is 1-based
    };

    const filteredSymbols = React.useMemo(() => {
        if (!profileData?.symbols?.symbols || !symbolsSearch.trim()) {
            return profileData?.symbols?.symbols || [];
        }
        const search = symbolsSearch.toLowerCase();
        return profileData.symbols.symbols.filter(symbol =>
            (symbol.name?.toLowerCase().includes(search)) ||
            (symbol.file?.toLowerCase().includes(search)) ||
            (symbol.kind?.toLowerCase().includes(search))
        );
    }, [profileData?.symbols?.symbols, symbolsSearch]);

    return (
        <Dialog open={open} onClose={onClose} maxWidth="lg" fullWidth>
            <DialogTitle>
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                    <Typography variant="h6">Project Profile</Typography>
                    {loading && <CircularProgress size={20} />}
                </Box>
            </DialogTitle>
            <DialogContent>
                <Box sx={{ borderBottom: 1, borderColor: 'divider' }}>
                    <Tabs value={tabValue} onChange={handleTabChange}>
                        <Tab label="Rules" />
                        <Tab label="Hotlist" />
                        <Tab label="Symbols" />
                        <Tab label="Overview" />
                    </Tabs>
                </Box>

                {/* Rules Tab */}
                <TabPanel value={tabValue} index={0}>
                    <Typography variant="h6" gutterBottom>Project Rules</Typography>
                    <Paper
                        variant="outlined"
                        sx={{
                            p: 2,
                            maxHeight: 400,
                            overflow: 'auto',
                            bgcolor: 'grey.900',
                            color: 'grey.100',
                            '& code': {
                                bgcolor: 'grey.800',
                                px: 0.5,
                                py: 0.25,
                                borderRadius: 0.5,
                                fontSize: '0.875rem'
                            },
                            '& h1, & h2, & h3, & h4, & h5, & h6': {
                                color: 'grey.200',
                                fontWeight: 600,
                                my: 1
                            },
                            '& ul, & ol': {
                                pl: 2
                            },
                            '& li': {
                                mb: 0.5
                            }
                        }}
                    >
                        <Typography
                            component="div"
                            sx={{
                                whiteSpace: 'pre-wrap',
                                fontFamily: 'monospace',
                                fontSize: '0.875rem',
                                lineHeight: 1.5,
                                color: 'inherit'
                            }}
                        >
                            {profileData?.rules || 'No rules available'}
                        </Typography>
                    </Paper>
                </TabPanel>

                {/* Hotlist Tab */}
                <TabPanel value={tabValue} index={1}>
                    <Typography variant="h6" gutterBottom>Important Files</Typography>
                    <Typography variant="body2" color="text.secondary" gutterBottom>
                        Files ranked by importance in the project
                    </Typography>
                    <Paper variant="outlined" sx={{ maxHeight: 400, overflow: 'auto' }}>
                        <List dense>
                            {(profileData?.hotlist || []).map((file, index) => (
                                <ListItem key={index} divider>
                                    <ListItemText
                                        primary={
                                            <Typography variant="body2" fontFamily="monospace">
                                                {file}
                                            </Typography>
                                        }
                                        secondary={`Rank #${index + 1}`}
                                    />
                                </ListItem>
                            ))}
                            {(!profileData?.hotlist || profileData.hotlist.length === 0) && (
                                <ListItem>
                                    <ListItemText primary="No hotlist available" />
                                </ListItem>
                            )}
                        </List>
                    </Paper>
                </TabPanel>

                {/* Symbols Tab */}
                <TabPanel value={tabValue} index={2}>
                    <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
                        <Typography variant="h6">Symbols</Typography>
                        <TextField
                            size="small"
                            placeholder="Search symbols..."
                            value={symbolsSearch}
                            onChange={(e) => setSymbolsSearch(e.target.value)}
                            sx={{ width: 250 }}
                        />
                    </Box>

                    <TableContainer component={Paper} variant="outlined" sx={{ maxHeight: 400 }}>
                        <Table stickyHeader size="small">
                            <TableHead>
                                <TableRow>
                                    <TableCell>Name</TableCell>
                                    <TableCell>Kind</TableCell>
                                    <TableCell>File</TableCell>
                                    <TableCell>Line</TableCell>
                                    <TableCell>Language</TableCell>
                                </TableRow>
                            </TableHead>
                            <TableBody>
                                {filteredSymbols.map((symbol, index) => (
                                    <TableRow key={symbol.id || index} hover>
                                        <TableCell>
                                            <Typography variant="body2" fontFamily="monospace">
                                                {symbol.name || 'N/A'}
                                            </Typography>
                                        </TableCell>
                                        <TableCell>
                                            <Chip
                                                label={symbol.kind || 'unknown'}
                                                size="small"
                                                variant="outlined"
                                                color={symbol.kind === 'func' ? 'primary' :
                                                    symbol.kind === 'class' ? 'secondary' : 'default'}
                                            />
                                        </TableCell>
                                        <TableCell>
                                            <Typography variant="body2" fontFamily="monospace" color="text.secondary">
                                                {symbol.file || 'N/A'}
                                            </Typography>
                                        </TableCell>
                                        <TableCell>
                                            <Typography variant="body2">
                                                {symbol.line || 'N/A'}
                                            </Typography>
                                        </TableCell>
                                        <TableCell>
                                            <Typography variant="body2">
                                                {symbol.lang || 'N/A'}
                                            </Typography>
                                        </TableCell>
                                    </TableRow>
                                ))}
                                {filteredSymbols.length === 0 && (
                                    <TableRow>
                                        <TableCell colSpan={5} align="center">
                                            <Typography color="text.secondary">
                                                {symbolsSearch ? 'No symbols match your search' : 'No symbols available'}
                                            </Typography>
                                        </TableCell>
                                    </TableRow>
                                )}
                            </TableBody>
                        </Table>
                    </TableContainer>

                    {!symbolsSearch && profileData?.symbols && profileData.symbols.total > symbolsLimit && (
                        <Box sx={{ display: 'flex', justifyContent: 'center', mt: 2 }}>
                            <Pagination
                                count={Math.ceil(profileData.symbols.total / symbolsLimit)}
                                page={symbolsPage + 1}
                                onChange={handleSymbolsPageChange}
                                color="primary"
                            />
                        </Box>
                    )}
                </TabPanel>

                {/* Overview Tab */}
                <TabPanel value={tabValue} index={3}>
                    <Typography variant="h6" gutterBottom>Project Overview</Typography>
                    {profileData?.profile?.exists ? (
                        <Box>
                            <Accordion>
                                <AccordionSummary expandIcon={<ExpandMoreIcon />}>
                                    <Typography variant="subtitle1">Languages & Entrypoints</Typography>
                                </AccordionSummary>
                                <AccordionDetails>
                                    <Box sx={{ display: 'flex', gap: 2, flexWrap: 'wrap', mb: 2 }}>
                                        {profileData.profile.profile?.languages?.map((lang: string) => (
                                            <Chip key={lang} label={lang} variant="outlined" />
                                        ))}
                                    </Box>
                                    <Typography variant="subtitle2" gutterBottom>Entry Points:</Typography>
                                    <List dense>
                                        {profileData.profile.profile?.entrypoints?.map((ep: any, index: number) => (
                                            <ListItem key={index}>
                                                <ListItemText
                                                    primary={ep.path || ep}
                                                    secondary={ep.kind}
                                                />
                                            </ListItem>
                                        ))}
                                    </List>
                                </AccordionDetails>
                            </Accordion>

                            <Accordion>
                                <AccordionSummary expandIcon={<ExpandMoreIcon />}>
                                    <Typography variant="subtitle1">Statistics</Typography>
                                </AccordionSummary>
                                <AccordionDetails>
                                    <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 2 }}>
                                        <Box>
                                            <Typography variant="body2" color="text.secondary">Total Files</Typography>
                                            <Typography variant="h6">{profileData.profile.profile?.file_count || 'N/A'}</Typography>
                                        </Box>
                                        <Box>
                                            <Typography variant="body2" color="text.secondary">Important Files</Typography>
                                            <Typography variant="h6">{profileData.hotlist?.length || 0}</Typography>
                                        </Box>
                                        <Box>
                                            <Typography variant="body2" color="text.secondary">Symbols</Typography>
                                            <Typography variant="h6">{profileData.symbols?.total || 0}</Typography>
                                        </Box>
                                        <Box>
                                            <Typography variant="body2" color="text.secondary">Confidence</Typography>
                                            <Typography variant="h6">
                                                {profileData.profile.profile?.confidence ?
                                                    `${(profileData.profile.profile.confidence * 100).toFixed(0)}%` : 'N/A'}
                                            </Typography>
                                        </Box>
                                    </Box>
                                </AccordionDetails>
                            </Accordion>
                        </Box>
                    ) : (
                        <Paper variant="outlined" sx={{ p: 3, textAlign: 'center' }}>
                            <Typography color="text.secondary">
                                No project profile available. Run profiling to generate project insights.
                            </Typography>
                        </Paper>
                    )}
                </TabPanel>
            </DialogContent>
            <DialogActions>
                <Button onClick={onClose} variant="contained">
                    Close
                </Button>
            </DialogActions>
        </Dialog>
    );
};

export default ProfileDialog;
