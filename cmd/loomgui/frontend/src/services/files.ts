// Minimal write API that uses the Wails bridge instead of HTTP
import * as Bridge from '../../wailsjs/go/bridge/App'

export async function writeFile(path: string, content: string, serverRev?: string) {
  // Prefer a native bridge method if available; otherwise, fall back to edit tool
  const payload: any = { path, content, serverRev: serverRev || '' }
  if (typeof (Bridge as any).WriteWorkspaceFile === 'function') {
    const res = await (Bridge as any).WriteWorkspaceFile(payload)
    return res as { serverRev: string }
  }
  // Fallback: use Edit tool simple replace full file content
  // This preserves the existing approval flow if enabled
  if (typeof (Bridge as any).ApplyEdit === 'function') {
    const res = await (Bridge as any).ApplyEdit({ path, content })
    return { serverRev: String(res?.serverRev || '') }
  }
  throw new Error('No save method available')
}


