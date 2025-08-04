import React, { useState } from 'react';
import './Layout.css';

interface LayoutProps {
  children: React.ReactNode;
  sidebar: React.ReactNode;
  header?: React.ReactNode;
  footer?: React.ReactNode;
}

export function Layout({ children, sidebar, header, footer }: LayoutProps) {
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);

  return (
    <div className="layout">
      {header && <header className="layout-header">{header}</header>}
      
      <div className="layout-main">
        <aside className={`layout-sidebar ${sidebarCollapsed ? 'collapsed' : ''}`}>
          <button 
            className="sidebar-toggle"
            onClick={() => setSidebarCollapsed(!sidebarCollapsed)}
            aria-label={sidebarCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}
          >
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d={sidebarCollapsed ? "m9 18 6-6-6-6" : "m15 18-6-6 6-6"} />
            </svg>
          </button>
          <div className="sidebar-content">
            {sidebar}
          </div>
        </aside>
        
        <main className="layout-content">
          {children}
        </main>
      </div>
      
      {footer && <footer className="layout-footer">{footer}</footer>}
    </div>
  );
}