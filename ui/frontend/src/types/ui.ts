export interface ChatMessage {
  role: string;
  content: string;
  id?: string;
}

export interface ApprovalRequest {
  id: string;
  summary: string;
  diff: string;
}

export interface UIFileEntry {
  name: string;
  path: string;
  is_dir: boolean;
  size?: number;
  mod_time: string;
}

export interface UIListDirResult {
  path: string;
  entries: UIFileEntry[];
  is_dir: boolean;
  error?: string;
}

export type LanguageId =
  | 'typescript' | 'javascript' | 'json' | 'yaml' | 'markdown'
  | 'go' | 'php' | 'python' | 'css' | 'html' | 'text'

export interface EditorTabItem {
  path: string;
  title: string;
  content: string;
  language?: LanguageId;
  isDirty?: boolean;
  version?: number;
  serverRev?: string;
  cursor?: { line: number; column: number };
  scrollTop?: number;
}

export interface ConversationListItem {
  id: string;
  title: string;
  updated_at?: string;
}


