import React, { useState } from 'react';
import { Layout } from './components/Layout';
import { ChatWindow } from './components/ChatWindow';
import { FileExplorer } from './components/FileExplorer';
import { TaskQueue } from './components/TaskQueue';
import { useApp } from './hooks/useWails';
import type { FileInfo, ViewState } from './types';
import './styles/globals.css';
import './App.css';

function App() {
  const { appInfo, systemErrors, clearErrors } = useApp();
  const [viewState, setViewState] = useState<ViewState>({
    currentView: 'chat',
    sidebarCollapsed: false,
    darkMode: false
  });
  const [selectedFile, setSelectedFile] = useState<FileInfo | null>(null);

  const handleFileSelect = (file: FileInfo) => {
    setSelectedFile(file);
    // TODO: Implement file preview/editing
    console.log('Selected file:', file);
  };

  const toggleDarkMode = () => {
    setViewState(prev => ({ ...prev, darkMode: !prev.darkMode }));
    document.documentElement.setAttribute(
      'data-theme', 
      viewState.darkMode ? 'light' : 'dark'
    );
  };

  const renderMainContent = () => {
    switch (viewState.currentView) {
      case 'files':
        return (
          <div className="main-content files-view">
            <FileExplorer onFileSelect={handleFileSelect} />
            {selectedFile && (
              <div className="file-preview">
                <h3>Selected: {selectedFile.name}</h3>
                <p>Path: {selectedFile.path}</p>
                <p>Size: {selectedFile.size} bytes</p>
                <p>Language: {selectedFile.language || 'Unknown'}</p>
              </div>
            )}
          </div>
        );
      case 'tasks':
        return <TaskQueue className="main-content" />;
      case 'chat':
      default:
        return <ChatWindow className="main-content" />;
    }
  };

  const renderSidebar = () => (
    <div className="sidebar">
      {/* Navigation */}
      <nav className="sidebar-nav">
        <button
          className={`nav-item ${viewState.currentView === 'chat' ? 'active' : ''}`}
          onClick={() => setViewState(prev => ({ ...prev, currentView: 'chat' }))}
        >
          <span className="nav-icon">💬</span>
          <span className="nav-label">Chat</span>
        </button>
        <button
          className={`nav-item ${viewState.currentView === 'files' ? 'active' : ''}`}
          onClick={() => setViewState(prev => ({ ...prev, currentView: 'files' }))}
        >
          <span className="nav-icon">📁</span>
          <span className="nav-label">Files</span>
        </button>
        <button
          className={`nav-item ${viewState.currentView === 'tasks' ? 'active' : ''}`}
          onClick={() => setViewState(prev => ({ ...prev, currentView: 'tasks' }))}
        >
          <span className="nav-icon">📋</span>
          <span className="nav-label">Tasks</span>
        </button>
      </nav>

      {/* Sidebar content based on current view */}
      <div className="sidebar-content-area">
        {viewState.currentView === 'chat' && <FileExplorer onFileSelect={handleFileSelect} />}
        {viewState.currentView === 'files' && <TaskQueue />}
        {viewState.currentView === 'tasks' && <FileExplorer onFileSelect={handleFileSelect} />}
      </div>
    </div>
  );

  const renderHeader = () => (
    <div className="app-header">
      <div className="header-left">
        <h1 className="app-title">Loom</h1>
        {appInfo && (
          <span className="workspace-path">{appInfo.workspacePath}</span>
        )}
      </div>
      <div className="header-right">
        <button onClick={toggleDarkMode} className="btn btn-ghost">
          {viewState.darkMode ? '☀️' : '🌙'}
        </button>
        {appInfo && !appInfo.hasLLM && (
          <span className="status-badge warning">No LLM</span>
        )}
      </div>
    </div>
  );

  return (
    <div className="app" data-theme={viewState.darkMode ? 'dark' : 'light'}>
      <Layout
        header={renderHeader()}
        sidebar={renderSidebar()}
      >
        {renderMainContent()}
      </Layout>
      
      {/* System errors overlay */}
      {systemErrors.length > 0 && (
        <div className="error-overlay">
          {systemErrors.map((error, index) => (
            <div key={index} className="error-toast">
              <span>{error}</span>
              <button onClick={clearErrors} className="error-close">×</button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

export default App;
