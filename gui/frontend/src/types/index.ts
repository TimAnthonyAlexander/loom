// Core data types matching the Go backend models

export interface Message {
  id: string;
  content: string;
  isUser: boolean;
  timestamp: number; // Unix timestamp in milliseconds
  type: string; // "user", "assistant", "system", "debug"
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
  modifiedTime: number; // Unix timestamp in milliseconds
}

export interface ProjectSummary {
  summary: string;
  languages: Record<string, number>; // language -> percentage
  fileCount: number;
  totalLines: number;
  generatedAt: number; // Unix timestamp in milliseconds
}

export interface TaskInfo {
  id: string;
  type: string;
  description: string;
  status: 'pending' | 'executing' | 'completed' | 'failed';
  createdAt: number; // Unix timestamp in milliseconds
  completedAt?: number;
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