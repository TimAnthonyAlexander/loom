import React, { useState, useEffect, useRef, ReactElement } from 'react';
import { EventsOn } from '../wailsjs/runtime/runtime';
import { SendUserMessage, Approve, GetTools } from '../wailsjs/go/bridge/App';
import './App.css';

// Helper function to format diff output with syntax highlighting
const formatDiff = (diff: string): ReactElement => {
  if (!diff) return <pre>No changes</pre>;

  // Split the diff into lines
  const lines = diff.split('\n');
  
  // Track if we're in the header section
  let inHeader = true;
  const headerLines: string[] = [];
  const contentLines: string[] = [];
  
  // Separate header from content
  for (const line of lines) {
    if (inHeader && (line.startsWith('---') || line.startsWith('+++'))) {
      headerLines.push(line);
    } else if (line === '') {
      inHeader = false;
    } else {
      contentLines.push(line);
    }
  }
  
  return (
    <>
      {headerLines.length > 0 && (
        <div className="diff-header">
          {headerLines.map((line, i) => <div key={`header-${i}`}>{line}</div>)}
        </div>
      )}
      
      <div className="diff-content">
        {contentLines.map((line, i) => {
          // Format line based on its prefix
          if (line.startsWith('+') || line.startsWith('+')) {
            const match = line.match(/^(\+)(\s*\d+:\s)(.*)$/);
            if (match) {
              return (
                <div key={`line-${i}`} className="diff-added">
                  <span className="diff-line-number">{match[2]}</span>
                  {match[3]}
                </div>
              );
            }
            
            return <div key={`line-${i}`} className="diff-added">{line}</div>;
          } else if (line.startsWith('-') || line.startsWith('-')) {
            const match = line.match(/^(\-)(\s*\d+:\s)(.*)$/);
            if (match) {
              return (
                <div key={`line-${i}`} className="diff-removed">
                  <span className="diff-line-number">{match[2]}</span>
                  {match[3]}
                </div>
              );
            }
            
            return <div key={`line-${i}`} className="diff-removed">{line}</div>;
          } else if (line.match(/^\d+ line\(s\) changed$/)) {
            return <div key={`line-${i}`} className="diff-summary">{line}</div>;
          }
          
          // Normal context lines
          const match = line.match(/^(\s)(\s*\d+:\s)(.*)$/);
          if (match) {
            return (
              <div key={`line-${i}`}>
                <span className="diff-line-number">{match[2]}</span>
                {match[3]}
              </div>
            );
          }
          
          return <div key={`line-${i}`}>{line}</div>;
        })}
      </div>
    </>
  );
};

// Define types for messages from backend
interface ChatMessage {
  role: string;
  content: string;
  id?: string;
}

interface ApprovalRequest {
  id: string;
  summary: string;
  diff: string;
}

interface Tool {
  name: string;
  description: string;
  safe: boolean;
}

const App: React.FC = () => {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [input, setInput] = useState('');
  const [approvalRequest, setApprovalRequest] = useState<ApprovalRequest | null>(null);
  const [tools, setTools] = useState<Tool[]>([]);
  const messagesEndRef = useRef<null | HTMLDivElement>(null);

  useEffect(() => {
    // Listen for new chat messages
    EventsOn('chat:new', (message: ChatMessage) => {
      setMessages(prev => [...prev, message]);
    });

    // Listen for approval requests
    EventsOn('task:prompt', (request: ApprovalRequest) => {
      setApprovalRequest(request);
    });

    // Get available tools
    GetTools().then((fetchedTools: Record<string, any>[]) => {
      const typedTools: Tool[] = fetchedTools.map(tool => ({
        name: tool.name || '',
        description: tool.description || '',
        safe: Boolean(tool.safe)
      }));
      setTools(typedTools);
    });
  }, []);

  // Scroll to bottom when messages change
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  const handleSend = () => {
    if (!input.trim()) return;
    
    // Send message to backend
    SendUserMessage(input);
    setInput('');
  };

  const handleApproval = (approved: boolean) => {
    if (approvalRequest) {
      Approve(approvalRequest.id, approved);
      setApprovalRequest(null);
    }
  };

  return (
    <div className="app">
      <div className="sidebar">
        <h1>Loom v2</h1>
        <div className="tools-list">
          <h2>Available Tools</h2>
          <ul>
            {tools.map(tool => (
              <li key={tool.name}>
                <strong>{tool.name}</strong>
                <p>{tool.description}</p>
                <span className={tool.safe ? 'safe' : 'requires-approval'}>
                  {tool.safe ? 'Safe' : 'Requires Approval'}
                </span>
              </li>
            ))}
          </ul>
        </div>
      </div>
      
      <div className="main">
        <div className="chat-window">
          {messages.map((msg, index) => (
            <div key={index} className={`message ${msg.role}`}>
              <div className="role">{msg.role}</div>
              <div className="content">{msg.content}</div>
            </div>
          ))}
          <div ref={messagesEndRef} />
        </div>
        
        <div className="input-area">
          <textarea 
            value={input} 
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                handleSend();
              }
            }}
            placeholder="Ask Loom anything..."
          />
          <button onClick={handleSend}>Send</button>
        </div>
      </div>

      {approvalRequest && (
        <div className="approval-modal">
          <div className="modal-content">
            <h2>Action Requires Approval</h2>
            <h3>{approvalRequest.summary}</h3>
            
            <div className="diff-view">
              {formatDiff(approvalRequest.diff)}
            </div>
            
            <div className="approval-actions">
              <button 
                className="reject" 
                onClick={() => handleApproval(false)}
              >
                Reject
              </button>
              <button 
                className="approve" 
                onClick={() => handleApproval(true)}
              >
                Approve
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default App;