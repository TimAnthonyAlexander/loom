import React, { useState } from 'react';
import { useTasks } from '../hooks/useWails';
import type { TaskInfo, TaskConfirmation } from '../types';
import './TaskQueue.css';

interface TaskQueueProps {
  className?: string;
}

export function TaskQueue({ className }: TaskQueueProps) {
  const { allTasks, pendingConfirmations, approveTask, rejectTask } = useTasks();
  const [selectedTab, setSelectedTab] = useState<'pending' | 'executing' | 'completed'>('pending');
  const [showConfirmations, setShowConfirmations] = useState(true);

  const getStatusIcon = (status: TaskInfo['status']): string => {
    switch (status) {
      case 'pending': return '⏳';
      case 'executing': return '⚡';
      case 'completed': return '✅';
      case 'failed': return '❌';
      default: return '❓';
    }
  };

  const getTaskTypeIcon = (type: string): string => {
    switch (type.toLowerCase()) {
      case 'read': return '👁️';
      case 'list': return '📋';
      case 'search': return '🔍';
      case 'run': return '🚀';
      case 'loom_edit': return '✏️';
      default: return '📄';
    }
  };

  const formatTimestamp = (timestamp: number) => {
    return new Date(timestamp).toLocaleString([], {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    });
  };

  const renderTaskItem = (task: TaskInfo) => (
    <div key={task.id} className={`task-item status-${task.status}`}>
      <div className="task-header">
        <div className="task-info">
          <span className="task-icon">{getTaskTypeIcon(task.type)}</span>
          <span className="task-type">{task.type}</span>
          <span className="task-status">
            {getStatusIcon(task.status)} {task.status}
          </span>
        </div>
        <span className="task-time">{formatTimestamp(task.createdAt)}</span>
      </div>
      
      <div className="task-description">{task.description}</div>
      
      {task.preview && (
        <details className="task-preview">
          <summary>Preview</summary>
          <pre className="preview-content">{task.preview}</pre>
        </details>
      )}
      
      {task.result && (
        <details className="task-result">
          <summary>Result</summary>
          <pre className="result-content">{task.result}</pre>
        </details>
      )}
      
      {task.error && (
        <div className="task-error">
          <strong>Error:</strong> {task.error}
        </div>
      )}
      
      {task.completedAt && (
        <div className="task-completion">
          Completed: {formatTimestamp(task.completedAt)}
        </div>
      )}
    </div>
  );

  const renderConfirmationItem = (confirmation: TaskConfirmation) => (
    <div key={confirmation.taskInfo.id} className="confirmation-item">
      <div className="confirmation-header">
        <div className="task-info">
          <span className="task-icon">{getTaskTypeIcon(confirmation.taskInfo.type)}</span>
          <span className="task-type">{confirmation.taskInfo.type}</span>
          <span className="confirmation-badge">Needs Approval</span>
        </div>
      </div>
      
      <div className="task-description">{confirmation.taskInfo.description}</div>
      
      {confirmation.preview && (
        <div className="confirmation-preview">
          <h5>Preview:</h5>
          <pre className="preview-content">{confirmation.preview}</pre>
        </div>
      )}
      
      <div className="confirmation-actions">
        <button
          onClick={() => approveTask(confirmation.taskInfo.id)}
          className="btn btn-primary btn-sm"
        >
          ✅ Approve
        </button>
        <button
          onClick={() => rejectTask(confirmation.taskInfo.id)}
          className="btn btn-secondary btn-sm"
        >
          ❌ Reject
        </button>
      </div>
    </div>
  );

  const currentTasks = allTasks[selectedTab] || [];
  const totalTasks = Object.values(allTasks).flat().length;

  return (
    <div className={`task-queue ${className || ''}`}>
      {/* Header */}
      <div className="task-queue-header">
        <h3>Tasks</h3>
        <div className="task-stats">
          <span className="text-sm text-secondary">
            {totalTasks} total
          </span>
        </div>
      </div>

      {/* Pending Confirmations */}
      {pendingConfirmations.length > 0 && (
        <div className="confirmations-section">
          <div 
            className="section-header clickable"
            onClick={() => setShowConfirmations(!showConfirmations)}
          >
            <h4>
              Pending Confirmations ({pendingConfirmations.length})
              <span className="expand-icon">{showConfirmations ? '▼' : '▶'}</span>
            </h4>
          </div>
          
          {showConfirmations && (
            <div className="confirmations-list">
              {pendingConfirmations.map(renderConfirmationItem)}
            </div>
          )}
        </div>
      )}

      {/* Task Tabs */}
      <div className="task-tabs">
        {(['pending', 'executing', 'completed'] as const).map(tab => (
          <button
            key={tab}
            onClick={() => setSelectedTab(tab)}
            className={`task-tab ${selectedTab === tab ? 'active' : ''}`}
          >
            {tab} ({(allTasks[tab] || []).length})
          </button>
        ))}
      </div>

      {/* Task List */}
      <div className="task-list">
        {currentTasks.length > 0 ? (
          currentTasks.map(renderTaskItem)
        ) : (
          <div className="empty-state">
            <div className="empty-icon">📋</div>
            <div className="empty-text">No {selectedTab} tasks</div>
          </div>
        )}
      </div>
    </div>
  );
}