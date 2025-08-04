import React, { useState, useEffect } from 'react';
import * as App from '../../wailsjs/go/main/App';
import './WorkspaceSelector.css';

interface WorkspaceSelectorProps {
  onWorkspaceSelected: (workspacePath: string) => void;
  autoTryLastWorkspace?: boolean;
}

const LAST_WORKSPACE_KEY = 'loom_last_workspace';

export function WorkspaceSelector({ onWorkspaceSelected, autoTryLastWorkspace = false }: WorkspaceSelectorProps) {
  const [selectedPath, setSelectedPath] = useState<string>('');
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string>('');
  const [hasTriedAutoLoad, setHasTriedAutoLoad] = useState(false);

  const handleOpenDialog = async () => {
    try {
      setError('');
      const path = await App.OpenDirectoryDialog();
      if (path) {
        setSelectedPath(path);
      }
    } catch (err) {
      setError('Failed to open directory dialog: ' + (err as Error).message);
    }
  };

  // Auto-load last workspace on mount
  useEffect(() => {
    if (autoTryLastWorkspace && !hasTriedAutoLoad) {
      setHasTriedAutoLoad(true);
      tryLoadLastWorkspace();
    }
  }, [autoTryLastWorkspace, hasTriedAutoLoad]);

  const tryLoadLastWorkspace = async () => {
    try {
      const lastWorkspace = localStorage.getItem(LAST_WORKSPACE_KEY);
      if (lastWorkspace) {
        setSelectedPath(lastWorkspace);
        setIsLoading(true);
        await App.SelectWorkspace(lastWorkspace);
        localStorage.setItem(LAST_WORKSPACE_KEY, lastWorkspace); // Update timestamp
        onWorkspaceSelected(lastWorkspace);
      }
    } catch (err) {
      console.log('Failed to auto-load last workspace:', err);
      // Clear invalid workspace from localStorage
      localStorage.removeItem(LAST_WORKSPACE_KEY);
      setError('Could not load last workspace. Please select a new one.');
    } finally {
      setIsLoading(false);
    }
  };

  const handleSelectWorkspace = async () => {
    if (!selectedPath) {
      setError('Please select a directory first');
      return;
    }

    setIsLoading(true);
    setError('');

    try {
      await App.SelectWorkspace(selectedPath);
      // Save to localStorage on successful workspace selection
      localStorage.setItem(LAST_WORKSPACE_KEY, selectedPath);
      onWorkspaceSelected(selectedPath);
    } catch (err) {
      setError('Failed to initialize workspace: ' + (err as Error).message);
    } finally {
      setIsLoading(false);
    }
  };

  const handlePathChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    setSelectedPath(event.target.value);
    setError('');
  };

  return (
    <div className="workspace-selector">
      <div className="workspace-selector-content">
        <div className="workspace-selector-header">
          <h1>Welcome to Loom</h1>
          <p>AI-powered coding assistant</p>
        </div>

        <div className="workspace-selector-form">
          <h2>Select Workspace Directory</h2>
          <p className="workspace-selector-description">
            Choose the directory you want to work with. Loom will index files and help you code in this workspace.
          </p>

          <div className="workspace-selector-input-group">
            <label htmlFor="workspace-path">Workspace Path:</label>
            <div className="workspace-selector-input-row">
              <input
                id="workspace-path"
                type="text"
                value={selectedPath}
                onChange={handlePathChange}
                placeholder="/path/to/your/project"
                className="workspace-selector-input"
                disabled={isLoading}
              />
              <button
                type="button"
                onClick={handleOpenDialog}
                className="workspace-selector-browse-btn"
                disabled={isLoading}
              >
                Browse...
              </button>
            </div>
          </div>

          {error && (
            <div className="workspace-selector-error">
              {error}
            </div>
          )}

          <div className="workspace-selector-actions">
            <button
              onClick={handleSelectWorkspace}
              disabled={!selectedPath || isLoading}
              className="workspace-selector-primary-btn"
            >
              {isLoading ? 'Initializing...' : 'Select Workspace'}
            </button>
          </div>

          <div className="workspace-selector-tips">
            <h3>Tips:</h3>
            <ul>
              <li>Choose a project directory containing source code</li>
              <li>Loom works best with Git repositories</li>
              <li>Avoid selecting system directories (/, /System, /usr)</li>
              <li>Make sure you have read/write permissions to the directory</li>
            </ul>
          </div>
        </div>
      </div>
    </div>
  );
}