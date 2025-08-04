import React, { useState, useMemo } from 'react';
import { useFiles } from '../hooks/useWails';
import type { FileInfo, FileTreeNode } from '../types';
import './FileExplorer.css';

interface FileExplorerProps {
  className?: string;
  onFileSelect?: (file: FileInfo) => void;
}

export function FileExplorer({ className, onFileSelect }: FileExplorerProps) {
  const { fileTree, projectSummary, searchFiles, isLoading } = useFiles();
  
  // Ensure fileTree is safe to use with proper null safety
  const safeFileTree = Array.isArray(fileTree) ? fileTree : [];
  const [searchQuery, setSearchQuery] = useState('');
  const [searchResults, setSearchResults] = useState<FileInfo[]>([]);
  const [expandedFolders, setExpandedFolders] = useState<Set<string>>(new Set());
  const [selectedFile, setSelectedFile] = useState<string | null>(null);

  // Build tree structure from flat file list
  const fileTreeNodes = useMemo(() => {
    const buildTree = (files: FileInfo[]): FileTreeNode[] => {
      const sortedFiles = [...files].sort((a, b) => {
        // Directories first, then files
        if (a.isDirectory !== b.isDirectory) {
          return a.isDirectory ? -1 : 1;
        }
        return a.name.localeCompare(b.name);
      });

      return sortedFiles.map(file => ({
        file,
        children: file.isDirectory ? buildTree(
          files.filter(f => f.path.startsWith(file.path + '/') && 
                            f.path.split('/').length === file.path.split('/').length + 1)
        ) : undefined,
        expanded: expandedFolders.has(file.path)
      }));
    };

    // Get root level files and directories
    const rootFiles = safeFileTree.filter(file => !file.path.includes('/'));
    return buildTree(rootFiles);
  }, [safeFileTree, expandedFolders]);

  const handleSearch = async () => {
    if (!searchQuery.trim()) {
      setSearchResults([]);
      return;
    }
    
    const results = await searchFiles(searchQuery);
    setSearchResults(results);
  };

  const toggleFolder = (path: string) => {
    const newExpanded = new Set(expandedFolders);
    if (newExpanded.has(path)) {
      newExpanded.delete(path);
    } else {
      newExpanded.add(path);
    }
    setExpandedFolders(newExpanded);
  };

  const handleFileClick = (file: FileInfo) => {
    if (file.isDirectory) {
      toggleFolder(file.path);
    } else {
      setSelectedFile(file.path);
      onFileSelect?.(file);
    }
  };

  const formatFileSize = (bytes: number): string => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  };

  const getFileIcon = (file: FileInfo): string => {
    if (file.isDirectory) {
      return expandedFolders.has(file.path) ? '📂' : '📁';
    }
    
    const ext = file.path.split('.').pop()?.toLowerCase();
    switch (ext) {
      case 'js':
      case 'jsx':
      case 'ts':
      case 'tsx':
        return '📄';
      case 'css':
      case 'scss':
      case 'sass':
        return '🎨';
      case 'html':
      case 'htm':
        return '🌐';
      case 'json':
        return '⚙️';
      case 'md':
      case 'markdown':
        return '📝';
      case 'png':
      case 'jpg':
      case 'jpeg':
      case 'gif':
      case 'svg':
        return '🖼️';
      default:
        return '📄';
    }
  };

  const renderFileNode = (node: FileTreeNode, level: number = 0): React.ReactNode => {
    const { file } = node;
    const isSelected = selectedFile === file.path;
    const hasChildren = node.children && node.children.length > 0;

    return (
      <div key={file.path} className="file-node">
        <div
          className={`file-item ${isSelected ? 'selected' : ''}`}
          style={{ paddingLeft: `${level * 20 + 8}px` }}
          onClick={() => handleFileClick(file)}
        >
          <span className="file-icon">{getFileIcon(file)}</span>
          <span className="file-name" title={file.path}>
            {file.name}
          </span>
          {!file.isDirectory && (
            <span className="file-size">{formatFileSize(file.size)}</span>
          )}
        </div>
        
        {file.isDirectory && node.expanded && hasChildren && (
          <div className="file-children">
            {node.children!.map(child => renderFileNode(child, level + 1))}
          </div>
        )}
      </div>
    );
  };

  const renderSearchResults = () => (
    <div className="search-results">
      <h4>Search Results ({searchResults.length})</h4>
      {searchResults.map(file => (
        <div
          key={file.path}
          className={`file-item ${selectedFile === file.path ? 'selected' : ''}`}
          onClick={() => handleFileClick(file)}
        >
          <span className="file-icon">{getFileIcon(file)}</span>
          <span className="file-name" title={file.path}>
            {file.path}
          </span>
          <span className="file-size">{formatFileSize(file.size)}</span>
        </div>
      ))}
    </div>
  );

  return (
    <div className={`file-explorer ${className || ''}`}>
      {/* Header */}
      <div className="file-explorer-header">
        <h3>Files</h3>
        <div className="file-stats">
          {projectSummary && (
            <span className="text-sm text-secondary">
              {projectSummary.fileCount} files
            </span>
          )}
        </div>
      </div>

      {/* Loading state */}
      {isLoading && (
        <div className="loading-state" style={{padding: '32px', textAlign: 'center'}}>
          <div>Loading files...</div>
        </div>
      )}

      {/* Empty state when no files and not loading */}
      {!isLoading && safeFileTree.length === 0 && (
        <div className="empty-state" style={{padding: '32px', textAlign: 'center', color: 'var(--color-text-muted)'}}>
          <div style={{fontSize: '48px', marginBottom: '16px'}}>📁</div>
          <h4>No Files Available</h4>
          <p>Make sure a workspace is properly selected and initialized.</p>
        </div>
      )}

      {/* Search - only show when we have files */}
      {!isLoading && safeFileTree.length > 0 && (
        <div className="file-search">
          <div className="search-input-container">
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
              placeholder="Search files..."
              className="search-input"
            />
            <button onClick={handleSearch} className="search-button">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <circle cx="11" cy="11" r="8" />
                <path d="m21 21-4.35-4.35" />
              </svg>
            </button>
          </div>
        </div>
      )}

      {/* Project Summary - only show when we have files */}
      {!isLoading && safeFileTree.length > 0 && projectSummary && (
        <div className="project-summary">
          <div className="summary-text">{projectSummary.summary}</div>
          <div className="language-breakdown">
            {Object.entries(projectSummary.languages)
              .sort(([,a], [,b]) => b - a)
              .slice(0, 5)
              .map(([lang, percent]) => (
                <div key={lang} className="language-item">
                  <span className="language-name">{lang}</span>
                  <span className="language-percent">{percent.toFixed(1)}%</span>
                </div>
              ))}
          </div>
        </div>
      )}

      {/* File Tree - only show when we have files and not loading */}
      {!isLoading && safeFileTree.length > 0 && (
        <div className="file-tree">
          {searchQuery.trim() && searchResults.length > 0 ? (
            renderSearchResults()
          ) : (
            <div className="file-tree-content">
              {fileTreeNodes.map(node => renderFileNode(node))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}