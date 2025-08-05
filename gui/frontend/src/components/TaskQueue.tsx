import React, { useState } from 'react';
import { useTasks } from '../hooks/useWails';
import type { TaskInfo, TaskConfirmation } from '../types';
import './TaskQueue.css';

interface TaskQueueProps {
    className?: string;
}

export function TaskQueue({ className }: TaskQueueProps) {
    const { allTasks, pendingConfirmations, approveTask, rejectTask } = useTasks();

    // Use optional chaining for null safety
    const [selectedTab, setSelectedTab] = useState<'pending' | 'executing' | 'completed'>('pending');
    const [showConfirmations, setShowConfirmations] = useState(true);

    // Check if we have any task data at all
    const hasAnyTasks = Object.keys(allTasks ?? {}).length > 0 || (pendingConfirmations?.length ?? 0) > 0;

    const getStatusIcon = (status: string): string => {
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

    const formatTimestamp = (timestamp: any) => {
        const date = timestamp ? new Date(timestamp) : new Date();
        return date.toLocaleString([], {
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
                    <span className="task-status">
                        {getStatusIcon(task.status)} {task.status}
                    </span>
                </div>
                <span className="task-time">{formatTimestamp(task.createdAt)}</span>
            </div>

            <div className="task-description">{task.description}</div>

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

            {/* Empty state when no tasks */}
            {!hasAnyTasks && (
                <div className="empty-state" style={{ padding: '32px', textAlign: 'center', color: 'var(--color-text-muted)' }}>
                    <div style={{ fontSize: '48px', marginBottom: '16px' }}>📋</div>
                    <h4>No Tasks Available</h4>
                    <p>Tasks will appear here once you start using the chat or when background processes run.</p>
                </div>
            )}

            {/* Pending Confirmations - only show when we have tasks */}
            {hasAnyTasks && pendingConfirmations?.length > 0 && (
                <div className="confirmations-section">
                    <div
                        className="section-header clickable"
                        onClick={() => setShowConfirmations(!showConfirmations)}
                    >
                        <h4>
                            Pending Confirmations ({pendingConfirmations?.length || 0})
                            <span className="expand-icon">{showConfirmations ? '▼' : '▶'}</span>
                        </h4>
                    </div>

                    {showConfirmations && pendingConfirmations && (
                        <div className="confirmations-list">
                            {pendingConfirmations.map(renderConfirmationItem)}
                        </div>
                    )}
                </div>
            )}

            {/* Task Tabs - only show when we have tasks */}
            {hasAnyTasks && (
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
            )}

            {/* Task List - only show when we have tasks */}
            {hasAnyTasks && (
                <div className="task-list">
                    {(currentTasks && currentTasks.length > 0) ? (
                        currentTasks.map(renderTaskItem)
                    ) : (
                        <div className="empty-state">
                            <div className="empty-icon">📋</div>
                            <div className="empty-text">No {selectedTab} tasks</div>
                        </div>
                    )}
                </div>
            )}
        </div>
    );
}
