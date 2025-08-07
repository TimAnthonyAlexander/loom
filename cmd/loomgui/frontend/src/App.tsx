import React, { useState, useEffect, useRef } from 'react';
import { EventsOn } from '../wailsjs/runtime/runtime';
import { SendUserMessage, Approve, GetTools } from '../wailsjs/go/bridge/App';
import './App.css';

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
    GetTools().then((fetchedTools: Tool[]) => {
      setTools(fetchedTools);
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
              <pre>{approvalRequest.diff}</pre>
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