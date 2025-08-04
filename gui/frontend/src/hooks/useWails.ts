import { useEffect, useState, useCallback } from 'react';
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime';
import * as App from '../../wailsjs/go/main/App';
import type { 
  ChatState, 
  FileInfo, 
  TaskInfo, 
  TaskConfirmation, 
  ProjectSummary, 
  AppInfo,
  ChatStreamChunk,
  SystemError 
} from '../types';

// Hook for managing chat state and operations
export function useChat() {
  const [chatState, setChatState] = useState<ChatState | null>(null);
  const [isLoading, setIsLoading] = useState(false);

  // Load initial chat state
  useEffect(() => {
    App.GetChatState().then(setChatState);
  }, []);

  // Listen to real-time chat events
  useEffect(() => {
    const unsubscribers = [
      EventsOn('chat:message', (message) => {
        setChatState(prev => prev ? {
          ...prev,
          messages: [...prev.messages, message]
        } : null);
      }),

      EventsOn('chat:stream_chunk', (data: ChatStreamChunk) => {
        setChatState(prev => prev ? {
          ...prev,
          streamingContent: data.full,
          isStreaming: true
        } : null);
      }),

      EventsOn('chat:stream_ended', () => {
        setChatState(prev => prev ? {
          ...prev,
          streamingContent: '',
          isStreaming: false
        } : null);
        // Refresh chat state to get the final message
        App.GetChatState().then(setChatState);
      })
    ];

    return () => unsubscribers.forEach(unsub => unsub());
  }, []);

  const sendMessage = useCallback(async (message: string) => {
    setIsLoading(true);
    try {
      await App.SendMessage(message);
    } catch (error) {
      console.error('Failed to send message:', error);
    } finally {
      setIsLoading(false);
    }
  }, []);

  const clearChat = useCallback(async () => {
    try {
      await App.ClearChat();
      setChatState(prev => prev ? { ...prev, messages: [] } : null);
    } catch (error) {
      console.error('Failed to clear chat:', error);
    }
  }, []);

  const stopStreaming = useCallback(async () => {
    try {
      await App.StopStreaming();
    } catch (error) {
      console.error('Failed to stop streaming:', error);
    }
  }, []);

  return {
    chatState,
    isLoading,
    sendMessage,
    clearChat,
    stopStreaming
  };
}

// Hook for managing file operations
export function useFiles() {
  const [fileTree, setFileTree] = useState<FileInfo[]>([]);
  const [projectSummary, setProjectSummary] = useState<ProjectSummary | null>(null);
  const [isLoading, setIsLoading] = useState(false);

  // Load initial data
  useEffect(() => {
    Promise.all([
      App.GetFileTree(),
      App.GetProjectSummary()
    ]).then(([files, summary]) => {
      setFileTree(files);
      setProjectSummary(summary);
    });
  }, []);

  // Listen to file tree updates
  useEffect(() => {
    const unsubscriber = EventsOn('file:tree_updated', (files: FileInfo[]) => {
      setFileTree(files);
    });

    return () => unsubscriber();
  }, []);

  const readFile = useCallback(async (path: string): Promise<string> => {
    try {
      return await App.ReadFile(path);
    } catch (error) {
      console.error('Failed to read file:', error);
      throw error;
    }
  }, []);

  const searchFiles = useCallback(async (pattern: string): Promise<FileInfo[]> => {
    setIsLoading(true);
    try {
      const results = await App.SearchFiles(pattern);
      return results;
    } catch (error) {
      console.error('Failed to search files:', error);
      return [];
    } finally {
      setIsLoading(false);
    }
  }, []);

  const getFileAutocomplete = useCallback(async (query: string): Promise<string[]> => {
    try {
      return await App.GetFileAutocomplete(query);
    } catch (error) {
      console.error('Failed to get file autocomplete:', error);
      return [];
    }
  }, []);

  return {
    fileTree,
    projectSummary,
    isLoading,
    readFile,
    searchFiles,
    getFileAutocomplete
  };
}

// Hook for managing task operations
export function useTasks() {
  const [allTasks, setAllTasks] = useState<Record<string, TaskInfo[]>>({});
  const [pendingConfirmations, setPendingConfirmations] = useState<TaskConfirmation[]>([]);

  // Load initial data
  useEffect(() => {
    Promise.all([
      App.GetAllTasks(),
      App.GetPendingConfirmations()
    ]).then(([tasks, confirmations]) => {
      setAllTasks(tasks);
      setPendingConfirmations(confirmations);
    });
  }, []);

  // Listen to task events
  useEffect(() => {
    const unsubscribers = [
      EventsOn('task:confirmation_needed', (confirmation: TaskConfirmation) => {
        setPendingConfirmations(prev => [...prev, confirmation]);
      }),

      EventsOn('task:status_changed', (taskInfo: TaskInfo) => {
        // Refresh all tasks when status changes
        App.GetAllTasks().then(setAllTasks);
      }),

      EventsOn('task:completed', () => {
        // Refresh tasks and confirmations
        Promise.all([
          App.GetAllTasks(),
          App.GetPendingConfirmations()
        ]).then(([tasks, confirmations]) => {
          setAllTasks(tasks);
          setPendingConfirmations(confirmations);
        });
      })
    ];

    return () => unsubscribers.forEach(unsub => unsub());
  }, []);

  const approveTask = useCallback(async (taskId: string) => {
    try {
      await App.ApproveTask(taskId);
      // Remove from pending confirmations
      setPendingConfirmations(prev => prev.filter(conf => conf.taskInfo.id !== taskId));
    } catch (error) {
      console.error('Failed to approve task:', error);
    }
  }, []);

  const rejectTask = useCallback(async (taskId: string) => {
    try {
      await App.RejectTask(taskId);
      // Remove from pending confirmations
      setPendingConfirmations(prev => prev.filter(conf => conf.taskInfo.id !== taskId));
    } catch (error) {
      console.error('Failed to reject task:', error);
    }
  }, []);

  return {
    allTasks,
    pendingConfirmations,
    approveTask,
    rejectTask
  };
}

// Hook for app information and system events
export function useApp() {
  const [appInfo, setAppInfo] = useState<AppInfo | null>(null);
  const [systemErrors, setSystemErrors] = useState<string[]>([]);

  useEffect(() => {
    App.GetAppInfo().then(setAppInfo);
  }, []);

  // Listen to system events
  useEffect(() => {
    const unsubscriber = EventsOn('system:error', (error: SystemError) => {
      setSystemErrors(prev => [...prev, error.error]);
      // Auto-remove errors after 5 seconds
      setTimeout(() => {
        setSystemErrors(prev => prev.filter(err => err !== error.error));
      }, 5000);
    });

    return () => unsubscriber();
  }, []);

  const clearErrors = useCallback(() => {
    setSystemErrors([]);
  }, []);

  return {
    appInfo,
    systemErrors,
    clearErrors
  };
}