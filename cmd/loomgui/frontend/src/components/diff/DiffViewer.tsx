import { Box, Typography } from '@mui/material';

type DiffViewerProps = {
    diff?: string;
};

export default function DiffViewer({ diff }: DiffViewerProps) {
    if (!diff) {
        return (
            <Typography variant="body2" sx={{ fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace' }}>
                No changes
            </Typography>
        );
    }

    const lines = diff.split('\n');
    let inHeader = true;
    const headerLines: string[] = [];
    const contentLines: string[] = [];

    for (const line of lines) {
        if (inHeader && (line.startsWith('---') || line.startsWith('+++'))) {
            headerLines.push(line);
        } else if (line === '') {
            inHeader = false;
        } else {
            contentLines.push(line);
        }
    }

    return (
        <Box>
            {headerLines.length > 0 && (
                <Box sx={{ color: 'text.secondary', mb: 1 }}>
                    {headerLines.map((line, i) => (
                        <Typography
                            key={`header-${i}`}
                            variant="caption"
                            component="div"
                            sx={{ fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace' }}
                        >
                            {line}
                        </Typography>
                    ))}
                </Box>
            )}
            <Box>
                {contentLines.map((line, i) => {
                    if (line.startsWith('+')) {
                        const match = line.match(/^(\+)(\s*\d+:\s)(.*)$/);
                        return (
                            <Box
                                key={`line-${i}`}
                                sx={{
                                    bgcolor: 'success.light',
                                    color: 'success.contrastText',
                                    px: 1,
                                    py: 0.25,
                                    borderRadius: 0.5,
                                    my: 0.25,
                                    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
                                }}
                            >
                                {match ? (
                                    <>
                                        <Box component="span" sx={{ opacity: 0.75, mr: 1 }}>
                                            {match[2]}
                                        </Box>
                                        {match[3]}
                                    </>
                                ) : (
                                    line
                                )}
                            </Box>
                        );
                    }

                    if (line.startsWith('-')) {
                        const match = line.match(/^(\-)(\s*\d+:\s)(.*)$/);
                        return (
                            <Box
                                key={`line-${i}`}
                                sx={{
                                    bgcolor: 'error.light',
                                    color: 'error.contrastText',
                                    px: 1,
                                    py: 0.25,
                                    borderRadius: 0.5,
                                    my: 0.25,
                                    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
                                }}
                            >
                                {match ? (
                                    <>
                                        <Box component="span" sx={{ opacity: 0.75, mr: 1 }}>
                                            {match[2]}
                                        </Box>
                                        {match[3]}
                                    </>
                                ) : (
                                    line
                                )}
                            </Box>
                        );
                    }

                    if (line.match(/^\d+ line\(s\) changed$/)) {
                        return (
                            <Typography key={`line-${i}`} variant="caption" color="text.secondary">
                                {line}
                            </Typography>
                        );
                    }

                    const match = line.match(/^(\s)(\s*\d+:\s)(.*)$/);
                    if (match) {
                        return (
                            <Box key={`line-${i}`} sx={{ fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace' }}>
                                <Box component="span" sx={{ opacity: 0.5, mr: 1 }}>
                                    {match[2]}
                                </Box>
                                {match[3]}
                            </Box>
                        );
                    }

                    return (
                        <Box key={`line-${i}`} sx={{ fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace' }}>
                            {line}
                        </Box>
                    );
                })}
            </Box>
        </Box>
    );
}


