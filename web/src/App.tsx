import './App.css'

function IconCode() {
    return (
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path d="M14.5 4.5L9.5 19.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
            <path d="M8 8L3.5 12L8 16" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
            <path d="M16 8L20.5 12L16 16" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
    )
}

function IconShield() {
    return (
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path d="M12 3L5 6V12C5 16.4183 8.13401 19.5 12 21C15.866 19.5 19 16.4183 19 12V6L12 3Z" stroke="currentColor" strokeWidth="1.5" />
            <path d="M9 12L11 14L15 10" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
    )
}

function IconDevice() {
    return (
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
            <rect x="3" y="4" width="18" height="14" rx="2" stroke="currentColor" strokeWidth="1.5" />
            <path d="M8 20H16" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        </svg>
    )
}

export default function App() {
    return (
        <div>
            {/* Nav */}
            <nav className="nav">
                <div className="container nav-inner">
                    <div className="brand">
                        <img src="/logo.png" alt="Loom logo" />
                        <span>Loom</span>
                    </div>
                    <div className="nav-links">
                        <a href="#top" className="muted">Home</a>
                        <a href="https://github.com/TimAnthonyAlexander/loom/releases/latest" className="btn primary">Download</a>
                    </div>
                </div>
            </nav>

            {/* Hero */}
            <header className="hero section">
                <div className="container center">
                    <p className="pill" style={{ marginBottom: 16 }}>Desktop • Code‑aware • Approvals‑first</p>
                    <h1>
                        Loom — <strong>Your code‑aware AI assistant</strong>
                    </h1>
                    <p className="hero-sub">
                        Build, refactor, and explore your projects with an AI that understands your codebase, respects your approvals,
                        and integrates into your workflow — no cloud lock‑in. With comprehensive tools, symbol indexing, MCP integration,
                        and reasoning‑aware streaming.
                    </p>
                    <p>
                        Connect to OpenAI, Anthropic, OpenRouter (thousands of models), or run locally with Ollama. It's up to you.
                    </p>
                    <div className="cta">
                        <a href="https://github.com/TimAnthonyAlexander/loom/releases/latest" className="btn primary">Download Loom</a>
                        <a href="https://github.com/TimAnthonyAlexander/loom" className="btn ghost">Source on GitHub</a>
                    </div>

                    <div className="hero-visual">
                        <img
                            src="/screenshot_1.png"
                            alt="Loom screenshot"
                            style={{
                                width: '100%',
                                borderRadius: 8,
                                boxShadow: '0 2px 10px rgba(0,0,0,0.1)',
                                objectFit: 'cover',
                                objectPosition: 'center'
                            }}
                        ></img>
                    </div>
                </div>
            </header>

            {/* Trusted by */}
            <section className="section trusted">
                <div className="container center">
                    <p className="muted">Trusted by developers building production software</p>
                    <div className="logo-row" aria-label="trusted logos">
                        <span className="logo-pill">Open‑source maintainers</span>
                        <span className="logo-pill">Full‑stack developers</span>
                        <span className="logo-pill">Platform & infrastructure teams</span>
                        <span className="logo-pill">DevOps & automation engineers</span>
                    </div>
                </div>
            </section>

            {/* Why Loom */}
            <section id="why" className="section">
                <div className="container">
                    <h2 className="center">Why Loom?</h2>
                    <div className="grid grid-3 why-grid">
                        <div className="why-card">
                            <div className="why-icon"><IconCode /></div>
                            <div>
                                <h3>Deeply code‑aware</h3>
                                <p>Symbol indexing, semantic search, project profiling, and 20+ specialized tools for precise code understanding.</p>
                            </div>
                        </div>
                        <div className="why-card">
                            <div className="why-icon"><IconShield /></div>
                            <div>
                                <h3>Approvals‑first safety</h3>
                                <p>Every destructive action requires explicit approval. Optional auto‑approval for trusted workflows.</p>
                            </div>
                        </div>
                        <div className="why-card">
                            <div className="why-icon"><IconDevice /></div>
                            <div>
                                <h3>Extensible & private</h3>
                                <p>MCP integration, local‑first architecture, and comprehensive model provider support including local inference.</p>
                            </div>
                        </div>
                    </div>
                </div>
            </section>

            {/* Features */}
            <section id="features" className="section">
                <div className="container grid" style={{ gap: 48 }}>
                    {/* 1 */}
                    <div className="feature">
                        <div>
                            <h2>Deeply understand your code</h2>
                            <p>Symbol indexing with SQLite + FTS search, semantic exploration via ripgrep, and heuristic project profiling. Find definitions, references, and get smart code neighborhoods with precision.</p>
                        </div>
                        <div className="visual">
                            <img src="/screenshot_1.png" alt="Monaco editor with search results" />
                        </div>
                    </div>

                    {/* 2 */}
                    <div className="feature">
                        <div>
                            <h2>Precise, minimal edits</h2>
                            <p>Review side‑by‑side diffs. Approve and apply with one click — keeping control in your hands.</p>
                        </div>
                        <div className="visual">
                            <pre>
                                <code>{`// Precise, minimal edits
// Loom proposes the smallest possible diff
@@ internal/tool/edit.go
- old line
+ new line

Approve → Apply → Done.`}</code>
                            </pre>
                        </div>
                    </div>

                    {/* 3 */}
                    <div className="feature">
                        <div>
                            <h2>Run commands, safely</h2>
                            <p>Need a migration, build, or quick script? Loom proposes shell commands, shows you exactly what will run, and executes only with your permission.</p>
                        </div>
                        <div className="visual">
                            <pre>
                                <code>{`$ make dev
Wails + Vite with live reload
✔ backend: running
✔ frontend: running
→ open http://localhost:5173`}</code>
                            </pre>
                        </div>
                    </div>

                    {/* 5 */}
                    <div className="feature">
                        <div>
                            <h2>Reasoning & cost insights</h2>
                            <p>Streaming reasoning summaries from supported models (o3/o4/GPT‑5), comprehensive cost tracking across providers, and transparent token usage per project.</p>
                        </div>
                        <div className="visual">
                            <pre>
                                <code>{`💭 Reasoning:
"Analyzing the error pattern...
 Considering async handling...
 Best approach: add await"

💰 Cost tracking:
Project: $2.34 | Session: $0.12
Input: 1.2K tokens | Output: 856`}</code>
                            </pre>
                        </div>
                    </div>

                    {/* 6 */}
                    <div className="feature">
                        <div>
                            <h2>Thousands of models</h2>
                            <p>OpenAI, Anthropic Claude, OpenRouter (thousands of models with real‑time pricing), and local Ollama. From flagship models to cost‑effective alternatives, reasoning‑aware to lightning‑fast responses.</p>
                        </div>
                        <div className="visual">
                            <pre>
                                <code>{`Model Categories:
• Flagship: Claude Opus 4, GPT‑5
• Reasoning: o3, o4‑mini  
• Fast: Claude Haiku, GPT‑4o‑mini
• Cheap: Llama 3.3, Gemma
• Local: Ollama (DeepSeek R1, etc.)

OpenRouter: 1000+ models`}</code>
                            </pre>
                        </div>
                    </div>

                    {/* 7 */}
                    <div className="feature">
                        <div>
                            <h2>Extensible with MCP</h2>
                            <p>Connect external tools via Model Context Protocol. Integrate Jira, Confluence, Git, cloud APIs, and custom tools while maintaining Loom's approval‑first safety model.</p>
                        </div>
                        <div className="visual">
                            <pre>
                                <code>{`// .loom/mcp.json
{
  "mcpServers": {
    "jira": {
      "command": "uvx",
      "args": ["mcp-atlassian", "--read-only"]
    }
  }
}

→ jira_search, jira_get_issue
→ confluence_search, get_page`}</code>
                            </pre>
                        </div>
                    </div>

                    {/* 8 */}
                    <div className="feature">
                        <div>
                            <h2>Built for developers</h2>
                            <p>Tabbed Monaco editor, rules system for consistent AI behavior, workspace‑scoped memory, and a calm, content‑forward Material UI. Everything you need, nothing you don't.</p>
                        </div>
                        <div className="visual">
                            <img src="/screenshot_1.png" alt="Tabbed editor and explorer" />
                        </div>
                    </div>
                </div>
            </section>

            <section className="section" id="under-the-hood"
                style={{ maxWidth: '100vw' }}
            >
                <div className="container grid grid-2">
                    <div>
                        <h2>Comprehensive tool registry</h2>
                        <ul className="list">
                            <li><strong>Code Exploration:</strong> read_file, list_dir, search_code</li>
                            <li><strong>Safe Editing:</strong> edit_file, apply_edit (with approval)</li>
                            <li><strong>Shell Execution:</strong> run_shell, apply_shell (with approval)</li>
                            <li><strong>Project Intelligence:</strong> get_project_profile, get_hotlist, explain_file_importance</li>
                            <li><strong>Symbol Tools:</strong> symbols_search, symbols_def, symbols_refs, symbols_neighborhood, symbols_outline</li>
                            <li><strong>Workflow Tools:</strong> http_request, memories, todo_list, user_choice</li>
                            <li><strong>MCP Integration:</strong> Dynamic external tool loading via Model Context Protocol</li>
                        </ul>
                    </div>
                    <div className="card">
                        <pre>
                            <code>{`// 20+ specialized tools
registry := ToolRegistry{
  • Code: search, read, edit
  • Symbols: def, refs, outline  
  • Project: profile, hotlist
  • Shell: propose, approve, exec
  • Memory: persist, recall
  • HTTP: requests, APIs
  • MCP: jira, confluence, git
}

// Approval‑first safety
if (tool.destructive) {
  await promptApproval()
}`}</code>
                        </pre>
                    </div>
                </div>
            </section>

            {/* Security */}
            <section id="security" className="section"
                style={{ maxWidth: '100vw' }}
            >
                <div className="container grid grid-2">
                    <div>
                        <h2>Security & privacy</h2>
                        <ul className="list">
                            <li>Your code stays yours</li>
                            <li>No unapproved edits or commands</li>
                            <li>Workspace‑confined execution</li>
                            <li>API keys stored locally with restrictive permissions</li>
                            <li>Privacy mode for local‑only inference</li>
                        </ul>
                    </div>
                    <div className="card">
                        <pre>
                            <code>{`// Approvals‑first
onProposedEdit(diff) {
  promptApproval(diff)
}

onProposedCommand(cmd) {
  promptApproval(cmd)
}`}</code>
                        </pre>
                    </div>
                </div>
            </section>

            {/* Getting started */}
            <section id="getting-started" className="section">
                <div className="container center">
                    <h2>Getting started</h2>
                    <p className="muted">Four steps to productive, safe assistance</p>
                    <div className="grid grid-4 roadmap" style={{ marginTop: 24 }}>
                        <div className="card">1. Download Loom</div>
                        <div className="card">2. Connect providers (OpenAI, Claude, OpenRouter, Ollama)</div>
                        <div className="card">3. Open workspace & configure MCP (optional)</div>
                        <div className="card">4. Chat • Search • Edit • Profile • Extend</div>
                    </div>
                    <div className="hero-visual" style={{ marginTop: 24 }}>
                        <img src="/screenshot_1.png" alt="Loom getting started" />
                    </div>
                </div>
            </section>

            {/* Roadmap */}
            <section id="roadmap" className="section">
                <div className="container center">
                    <h2>Roadmap highlights</h2>
                    <div className="roadmap">
                        <div className="card">Multi‑file refactors & expanded toolset</div>
                        <div className="card">Vector‑backed memory & richer recall</div>
                        <div className="card">Granular approvals & audit trail</div>
                        <div className="card">Theming toggle & accessibility</div>
                        <div className="card">Enhanced provider streaming</div>
                    </div>
                </div>
            </section>

            {/* Download */}
            <section id="download" className="section">
                <div className="container center">
                    <h2>Ready to build faster — safely?</h2>
                    <p className="hero-sub">Get Loom and keep your workflow local, precise, and under control.</p>
                    <div className="cta">
                        <a className="btn primary" href="https://github.com/TimAnthonyAlexander/loom/releases/latest">Download for Free</a>
                        <a className="btn ghost" href="https://github.com/TimAnthonyAlexander/loom">Read the Docs</a>
                    </div>
                </div>
            </section>

            {/* Footer */}
            <footer>
                <div className="container">
                    <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12, flexWrap: 'wrap' }}>
                        <div className="brand"><img src="/logo.png" alt="Loom" /><span>Loom</span></div>
                        <nav className="nav-links" style={{ padding: 0 }}>
                            <a href="https://github.com/TimAnthonyAlexander/loom">GitHub</a>
                        </nav>
                    </div>
                    <p className="muted" style={{ marginTop: 12 }}>Loom is open for contributions. See our GitHub.</p>
                </div>
            </footer>
        </div>
    )
}
