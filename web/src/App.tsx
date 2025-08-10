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
                        <a href="#features" className="muted">Features</a>
                        <a href="#security" className="muted">Security</a>
                        <a href="https://github.com/TimAnthonyAlexander/loom" className="muted">Docs</a>
                        <a href="https://github.com/TimAnthonyAlexander/loom/releases/latest" className="btn primary">Download</a>
                    </div>
                </div>
            </nav>

            {/* Hero */}
            <header className="hero section">
                <div className="container center">
                    <p className="pill" style={{ marginBottom: 16 }}>Desktop • Code‑aware • Approvals‑first</p>
                    <h1>
                        Loom — <strong>Your code‑aware AI assistant</strong>, on your desktop.
                    </h1>
                    <p className="hero-sub">
                        Build, refactor, and explore your projects with an AI that understands your codebase, respects your approvals,
                        and integrates into your workflow — no cloud lock‑in.
                    </p>
                    <div className="cta">
                        <a href="https://github.com/TimAnthonyAlexander/loom/releases/latest" className="btn primary">Download Loom</a>
                        <a href="https://github.com/TimAnthonyAlexander/loom" className="btn ghost">Read the Docs</a>
                    </div>

                    <div className="hero-visual">
                        <img
                            src="/screenshot1.png"
                            alt="Loom screenshot"
                            style={{ width: '100%', maxWidth: 800, borderRadius: 8, boxShadow: '0 2px 10px rgba(0,0,0,0.1)' }}
                        />
                    </div>
                </div>
            </header>

            {/* Trusted by */}
            <section className="section trusted">
                <div className="container center">
                    <p className="muted">Trusted by developers who build serious software</p>
                    <div className="logo-row" aria-label="trusted logos">
                        <span className="logo-pill">Open‑source maintainers</span>
                        <span className="logo-pill">Indie devs</span>
                        <span className="logo-pill">Internal tools teams</span>
                        <span className="logo-pill">Data & infra engineers</span>
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
                                <h3>Code‑aware</h3>
                                <p>Loom reads, searches, and edits your code with precision.</p>
                            </div>
                        </div>
                        <div className="why-card">
                            <div className="why-icon"><IconShield /></div>
                            <div>
                                <h3>Safe</h3>
                                <p>Every change and shell command requires your approval.</p>
                            </div>
                        </div>
                        <div className="why-card">
                            <div className="why-icon"><IconDevice /></div>
                            <div>
                                <h3>Local‑first</h3>
                                <p>Runs on your machine with full workspace context. No lock‑in.</p>
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
                            <h2>Explore your codebase</h2>
                            <p>Semantic search powered by ripgrep. Ask Loom questions, jump to exact lines, and navigate dependencies without losing focus.</p>
                        </div>
                        <div className="visual">
                            <img src="/screenshot1.png" alt="Monaco editor with search results" />
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
                            <h2>Built for developers</h2>
                            <p>Tabbed Monaco editor, file explorer, theming, and a calm, content‑forward UI. Everything you need, nothing you don’t.</p>
                        </div>
                        <div className="visual">
                            <img src="/screenshot1.png" alt="Tabbed editor and explorer" />
                        </div>
                    </div>
                </div>
            </section>

            {/* Under the hood */}
            <section className="section" id="under-the-hood">
                <div className="container grid grid-2">
                    <div>
                        <h2>Under the hood</h2>
                        <ul className="list">
                            <li>Go + Wails backend — native app performance, cross‑platform packaging</li>
                            <li>React + TypeScript frontend — familiar, customizable interface</li>
                            <li>Explicit tool registry — extend Loom with safe or destructive tools</li>
                            <li>Memory & indexing — per‑workspace persistence for predictable behavior</li>
                        </ul>
                    </div>
                    <div className="card">
                        <pre>
                            <code>{`tools: [
  read_file(), search_code(), list_dir(),
  edit_file() → approval → apply_edit(),
  run_shell() → approval → apply_shell(),
  http_request(), finalize()
]`}</code>
                        </pre>
                    </div>
                </div>
            </section>

            {/* Security */}
            <section id="security" className="section">
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
  showDiffApprovalDialog(diff)
}

onProposedCommand(cmd) {
  showCommandApprovalDialog(cmd)
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
                        <div className="card">2. Connect model provider(s)</div>
                        <div className="card">3. Open a workspace</div>
                        <div className="card">4. Chat • Search • Edit • Run</div>
                    </div>
                    <div className="hero-visual" style={{ marginTop: 24 }}>
                        <img src="/screenshot1.png" alt="Loom getting started" />
                    </div>
                </div>
            </section>

            {/* Roadmap */}
            <section id="roadmap" className="section">
                <div className="container center">
                    <h2>Roadmap highlights</h2>
                    <div className="roadmap">
                        <div className="card">Multi‑file refactors</div>
                        <div className="card">Vector‑powered recall</div>
                        <div className="card">Granular approvals</div>
                        <div className="card">Custom theming</div>
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
