import React from 'react';
import ArchiveIcon from '@mui/icons-material/Archive';
import MusicNoteIcon from '@mui/icons-material/MusicNote';
import MovieIcon from '@mui/icons-material/Movie';
import TableChartIcon from '@mui/icons-material/TableChart';
import TerminalIcon from '@mui/icons-material/Terminal';
import HtmlIcon from '@mui/icons-material/Html';
import CssIcon from '@mui/icons-material/Css';
import JavascriptIcon from '@mui/icons-material/Javascript';
import PhpIcon from '@mui/icons-material/Php';
import StorageIcon from '@mui/icons-material/Storage';
import { Box, List, ListItemButton, ListItemIcon, ListItemText, Typography } from '@mui/material';
import { alpha } from '@mui/material/styles';
import ChevronRightIcon from '@mui/icons-material/ChevronRight';
import ExpandMoreIcon from '@mui/icons-material/ExpandMore';
import FolderIcon from '@mui/icons-material/Folder';
import FolderOpenIcon from '@mui/icons-material/FolderOpen';
import InsertDriveFileIcon from '@mui/icons-material/InsertDriveFile';
import DescriptionIcon from '@mui/icons-material/Description';
import CodeIcon from '@mui/icons-material/Code';
import ImageIcon from '@mui/icons-material/Image';
import DataObjectIcon from '@mui/icons-material/DataObject';
import PictureAsPdfIcon from '@mui/icons-material/PictureAsPdf';
import { UIFileEntry } from '../../../types/ui';

type Props = {
    dirCache: Record<string, UIFileEntry[]>;
    expandedDirs: Record<string, boolean>;
    onToggleDir: (path: string) => void;
    onOpenFile: (path: string) => void;
    rootPath?: string;
};

export default function FileExplorer({ dirCache, expandedDirs, onToggleDir, onOpenFile, rootPath = '' }: Props) {
    // Build a flattened list of the visible items for rendering and keyboard navigation
    type VisibleItem = {
        type: 'entry' | 'loading';
        path: string;
        name: string;
        isDir: boolean;
        depth: number;
    };

    const sortEntries = (entries: UIFileEntry[]) => {
        return [...entries].sort((a, b) => {
            if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1; // Folders first
            return a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
        });
    };

    const buildVisible = (startPath: string): { items: VisibleItem[]; pathToEntry: Record<string, UIFileEntry> } => {
        const items: VisibleItem[] = [];
        const pathToEntry: Record<string, UIFileEntry> = {};

        const walk = (dirPath: string, depth: number) => {
            const key = dirPath || '';
            const entries = sortEntries(dirCache[key] || []);
            for (const entry of entries) {
                pathToEntry[entry.path] = entry;
                items.push({ type: 'entry', path: entry.path, name: entry.name, isDir: entry.is_dir, depth });
                if (entry.is_dir && expandedDirs[entry.path]) {
                    if (dirCache[entry.path]) {
                        walk(entry.path, depth + 1);
                    } else {
                        // Loading placeholder when folder is expanded but children not yet loaded
                        items.push({ type: 'loading', path: `${entry.path}__loading`, name: 'Loading…', isDir: false, depth: depth + 1 });
                    }
                }
            }
        };

        walk(startPath || '', 0);
        return { items, pathToEntry };
    };

    const { items: visibleItems, pathToEntry } = buildVisible(rootPath);

    const getParentPath = (fullPath: string) => {
        const idx = fullPath.lastIndexOf('/');
        if (idx <= 0) return '';
        return fullPath.slice(0, idx);
    };

    const getFileIcon = (name: string, isDir?: boolean) => {
        if (isDir) return FolderIcon;
        const lower = name.toLowerCase();

        // Docs / text
        if (/(readme|license|changelog)(\.|$)/i.test(name)) return DescriptionIcon;
        if (lower.endsWith('.md') || lower.endsWith('.txt') || lower.endsWith('.rtf')) return DescriptionIcon;
        if (lower.endsWith('.pdf')) return PictureAsPdfIcon;
        if (lower.endsWith('.doc') || lower.endsWith('.docx') || lower.endsWith('.odt')) return DescriptionIcon;
        if (lower.endsWith('.xls') || lower.endsWith('.xlsx') || lower.endsWith('.ods') || lower.endsWith('.csv')) return TableChartIcon;
        if (lower.endsWith('.ppt') || lower.endsWith('.pptx') || lower.endsWith('.odp')) return DescriptionIcon;

        // Code
        if (lower.endsWith('.ts') || lower.endsWith('.tsx')) return CodeIcon;
        if (lower.endsWith('.js') || lower.endsWith('.jsx')) return JavascriptIcon;
        if (lower.endsWith('.html') || lower.endsWith('.htm')) return HtmlIcon;
        if (lower.endsWith('.css') || lower.endsWith('.scss') || lower.endsWith('.sass') || lower.endsWith('.less')) return CssIcon;
        if (lower.endsWith('.php')) return PhpIcon;
        if (lower.endsWith('.json') || lower.endsWith('.yaml') || lower.endsWith('.yml') || lower.endsWith('.xml')) return DataObjectIcon;
        if (lower.endsWith('.sh') || lower.endsWith('.bat') || lower.endsWith('.ps1')) return TerminalIcon;
        if (lower.endsWith('.sql') || lower.endsWith('.db') || lower.endsWith('.sqlite')) return StorageIcon;

        // Media
        if (lower.endsWith('.png') || lower.endsWith('.jpg') || lower.endsWith('.jpeg') || lower.endsWith('.gif') || lower.endsWith('.svg') || lower.endsWith('.webp') || lower.endsWith('.bmp') || lower.endsWith('.tiff')) return ImageIcon;
        if (lower.endsWith('.mp3') || lower.endsWith('.wav') || lower.endsWith('.ogg') || lower.endsWith('.flac') || lower.endsWith('.aac')) return MusicNoteIcon;
        if (lower.endsWith('.mp4') || lower.endsWith('.avi') || lower.endsWith('.mkv') || lower.endsWith('.mov') || lower.endsWith('.webm')) return MovieIcon;

        // Archives
        if (lower.endsWith('.zip') || lower.endsWith('.rar') || lower.endsWith('.7z') || lower.endsWith('.tar') || lower.endsWith('.gz') || lower.endsWith('.bz2')) return ArchiveIcon;

        return InsertDriveFileIcon;
    };

    const [selectedPath, setSelectedPath] = React.useState<string | null>(null);

    const findIndex = (path: string | null) => {
        if (!path) return -1;
        return visibleItems.findIndex((i) => i.type === 'entry' && i.path === path);
    };

    const handleKeyDown: React.KeyboardEventHandler<HTMLDivElement> = (e) => {
        if (!visibleItems.length) return;
        let currentIndex = findIndex(selectedPath);

        if (e.key === 'ArrowDown') {
            e.preventDefault();
            const nextIndex = Math.min(visibleItems.length - 1, Math.max(0, currentIndex + 1));
            const next = visibleItems[nextIndex];
            if (next.type === 'entry') setSelectedPath(next.path);
            return;
        }
        if (e.key === 'ArrowUp') {
            e.preventDefault();
            const prevIndex = Math.max(0, currentIndex === -1 ? 0 : currentIndex - 1);
            const prev = visibleItems[prevIndex];
            if (prev.type === 'entry') setSelectedPath(prev.path);
            return;
        }
        if (e.key === 'ArrowRight' || e.key === 'Enter') {
            e.preventDefault();
            // If nothing selected, select first
            if (currentIndex === -1) {
                const first = visibleItems.find((i) => i.type === 'entry');
                if (first && first.type === 'entry') setSelectedPath(first.path);
                return;
            }
            const current = visibleItems[currentIndex] as VisibleItem;
            if (current.type === 'entry') {
                const entry = pathToEntry[current.path];
                if (entry?.is_dir) {
                    if (!expandedDirs[entry.path]) {
                        onToggleDir(entry.path);
                    } else if (e.key === 'ArrowRight') {
                        // move to first child if any
                        const next = visibleItems[currentIndex + 1];
                        if (next && next.type === 'entry') setSelectedPath(next.path);
                    }
                } else {
                    onOpenFile(entry.path);
                }
            }
            return;
        }
        if (e.key === 'ArrowLeft') {
            e.preventDefault();
            if (currentIndex === -1) return;
            const current = visibleItems[currentIndex] as VisibleItem;
            if (current.type !== 'entry') return;
            const entry = pathToEntry[current.path];
            if (entry?.is_dir && expandedDirs[entry.path]) {
                onToggleDir(entry.path);
            } else {
                const parent = getParentPath(entry.path);
                // Find parent directory row
                const parentIndex = visibleItems.findIndex((i) => i.type === 'entry' && i.path === parent);
                if (parentIndex !== -1) setSelectedPath(parent);
            }
            return;
        }
        if (e.key === ' ') {
            e.preventDefault();
            if (currentIndex === -1) return;
            const current = visibleItems[currentIndex] as VisibleItem;
            if (current.type !== 'entry') return;
            const entry = pathToEntry[current.path];
            if (!entry) return;
            if (entry.is_dir) onToggleDir(entry.path);
            else onOpenFile(entry.path);
            return;
        }
    };

    return (
        <Box
            tabIndex={0}
            onKeyDown={handleKeyDown}
            sx={{
                outline: 'none',
                borderRadius: 1,
                '&:focus': {
                    boxShadow: (t) => `inset 0 0 0 1px ${alpha(t.palette.primary.main, 0.4)}`,
                },
            }}
        >
            <List dense disablePadding>
                {visibleItems.map((item) => {
                    if (item.type === 'loading') {
                        return (
                            <Box key={item.path} sx={{ pl: (item.depth + 2) * 1 }}>
                                <Typography variant="caption" color="text.secondary">Loading…</Typography>
                            </Box>
                        );
                    }
                    const isOpen = item.isDir && !!expandedDirs[item.path];
                    const isSelected = selectedPath === item.path;
                    const FileIcon = item.isDir
                        ? (isOpen ? FolderOpenIcon : FolderIcon)
                        : getFileIcon(item.name);
                    const Chevron = item.isDir ? (isOpen ? ExpandMoreIcon : ChevronRightIcon) : null;
                    return (
                        <ListItemButton
                            key={item.path}
                            selected={isSelected}
                            onClick={() => {
                                setSelectedPath(item.path);
                                if (item.isDir) onToggleDir(item.path);
                                else onOpenFile(item.path);
                            }}
                            sx={{
                                py: 0.25,
                                pl: 0.5,
                                pr: 1,
                                '&.Mui-selected': {
                                    bgcolor: (t) => alpha(t.palette.primary.main, 0.12),
                                },
                            }}
                            title={item.path}
                        >
                            <Box sx={{ display: 'flex', alignItems: 'center', width: '100%' }}>
                                <Box sx={{ width: (item.depth + 1) * 12, flex: '0 0 auto' }} />
                                <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, minWidth: 0 }}>
                                    <Box sx={{ width: 18, display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'text.secondary' }}>
                                        {Chevron ? <Chevron fontSize="small" /> : <span style={{ width: 18 }} />}
                                    </Box>
                                    <ListItemIcon sx={{ minWidth: 20, color: item.isDir ? 'text.secondary' : 'text.disabled' }}>
                                        <FileIcon fontSize="small" />
                                    </ListItemIcon>
                                    <ListItemText
                                        primaryTypographyProps={{
                                            fontFamily: 'ui-monospace, Menlo, monospace',
                                            fontSize: 13,
                                            whiteSpace: 'nowrap',
                                            overflow: 'hidden',
                                            textOverflow: 'ellipsis',
                                        }}
                                        primary={item.name}
                                    />
                                </Box>
                            </Box>
                        </ListItemButton>
                    );
                })}
            </List>
        </Box>
    );
}


