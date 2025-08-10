import type { LanguageId } from '../types/ui'

export function guessLanguage(path: string): LanguageId {
  const ext = path.split('.').pop()?.toLowerCase() || ''
  if (ext === 'ts' || ext === 'tsx') return 'typescript'
  if (ext === 'js' || ext === 'jsx' || ext === 'mjs' || ext === 'cjs') return 'javascript'
  if (ext === 'json') return 'json'
  if (ext === 'yml' || ext === 'yaml') return 'yaml'
  if (ext === 'md' || ext === 'mdx') return 'markdown'
  if (ext === 'go') return 'go'
  if (ext === 'php') return 'php'
  if (ext === 'py') return 'python'
  if (ext === 'css' || ext === 'scss' || ext === 'less') return 'css'
  if (ext === 'html' || ext === 'htm') return 'html'
  return 'text'
}


