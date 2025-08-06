// Core data types matching the Go backend models

export interface Message {
  id: string;
  content: string;
  isUser: boolean;
  timestamp: any; // Matches Wails generated type
  type: string; // "user", "assistant", "system", "debug"
  visible?: boolean; // Whether the message should be displayed in the UI
}

export interface ChatState {
  messages: Message[];
  isStreaming: boolean;
  streamingContent?: string;
  sessionId: string;
  workspacePath: string;
}

export interface FileInfo {
  path: string;
  name: string;
  size: number;
  isDirectory: boolean;
  language: string;
  modifiedTime: any; // Changed to any to match Wails generated types
}

export interface ProjectSummary {
  summary: string;
  languages: Record<string, number>; // language -> percentage
  fileCount: number;
  totalLines: number;
  generatedAt: any; // Changed to any to match Wails generated types
}

export interface TaskInfo {
  id: string;
  type: string;
  description: string;
  status: string; // Changed from union type to string to match Wails
  createdAt: any; // Changed to any to match Wails generated types
  completedAt?: any;
  error?: string;
  preview?: string;
  result?: string;
}

export interface TaskConfirmation {
  taskInfo: TaskInfo;
  preview: string;
  approved: boolean;
}

export interface AppInfo {
  workspacePath: string;
  version: string;
  hasLLM: boolean;
  workspaceInitialized: boolean;
}

// Event types for real-time updates
export interface ChatStreamChunk {
  content: string;
  full: string;
}

export interface SystemError {
  error: string;
}

// UI State types
export interface ViewState {
  currentView: 'chat' | 'files' | 'tasks';
  sidebarCollapsed: boolean;
  darkMode: boolean;
}

export interface FileTreeNode {
  file: FileInfo;
  children?: FileTreeNode[];
  expanded?: boolean;
}