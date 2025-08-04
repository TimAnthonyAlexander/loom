import React, { useState } from 'react';
import * as App from '../../wailsjs/go/main/App';
import './WorkspaceSelector.css';

interface WorkspaceSelectorProps {
  onWorkspaceSelected: (workspacePath: string) => void;
}

export function WorkspaceSelector({ onWorkspaceSelected }: WorkspaceSelectorProps) {
  const [selectedPath, setSelectedPath] = useState<string>('');
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string>('');

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

  const handleSelectWorkspace = async () => {
    if (!selectedPath) {
      setError('Please select a directory first');
      return;
    }

    setIsLoading(true);
    setError('');

    try {
      await App.SelectWorkspace(selectedPath);
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