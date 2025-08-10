import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import remarkBreaks from 'remark-breaks';
import { PrismLight as SyntaxHighlighter } from 'react-syntax-highlighter';
import { oneDark as oneDarkStyle } from 'react-syntax-highlighter/dist/esm/styles/prism';
import {
    Box,
    Paper,
    Table as MuiTable,
    TableCell as MuiTableCell,
    TableRow as MuiTableRow,
    TableContainer as MuiTableContainer,
} from '@mui/material';

const CustomTable = ({ children }: any) => (
    <MuiTableContainer component={Paper} variant="outlined" sx={{ borderRadius: 1 }}>
        <MuiTable size="small">{children}</MuiTable>
    </MuiTableContainer>
);

const CustomTableRow = ({ children, ...props }: any) => (
    <MuiTableRow {...props}>{children}</MuiTableRow>
);

const CustomTableCell = ({ children, ...props }: any) => (
    <MuiTableCell {...props}>{children}</MuiTableCell>
);

const CustomTableHeader = ({ children, ...props }: any) => (
    <MuiTableCell {...props} component="th">
        {children}
    </MuiTableCell>
);

type MarkdownRendererProps = {
    children: string;
};

export default function MarkdownRenderer({ children }: MarkdownRendererProps) {
    return (
        <ReactMarkdown
            remarkPlugins={[remarkGfm, remarkBreaks]}
            components={{
                code({ inline, className, children, ...props }: any) {
                    const match = /language-(\w+)/.exec(className || '');
                    return !inline && match ? (
                        <SyntaxHighlighter
                            style={oneDarkStyle as any}
                            language={match[1]}
                            PreTag="div"
                            customStyle={{ fontSize: 12, lineHeight: 1.5 }}
                        >
                            {String(children).replace(/\n$/, '')}
                        </SyntaxHighlighter>
                    ) : (
                        <Box
                            component="code"
                            className={className}
                            sx={{
                                bgcolor: 'action.hover',
                                borderRadius: 1,
                                px: 0.5,
                                py: 0.25,
                                fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
                                fontSize: 12,
                            }}
                            {...props}
                        >
                            {children}
                        </Box>
                    );
                },
                table: CustomTable as any,
                tr: CustomTableRow as any,
                td: CustomTableCell as any,
                th: CustomTableHeader as any,
            }}
        >
            {children}
        </ReactMarkdown>
    );
}


