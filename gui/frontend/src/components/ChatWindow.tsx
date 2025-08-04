import React, { useState, useRef, useEffect } from 'react';
import { useChat } from '../hooks/useWails';
import type { Message } from '../types';
import './ChatWindow.css';

interface ChatWindowProps {
  className?: string;
}

export function ChatWindow({ className }: ChatWindowProps) {
  const { chatState, isLoading, sendMessage, clearChat, stopStreaming } = useChat();
  const [inputValue, setInputValue] = useState('');
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);

  // Auto-scroll to bottom when new messages arrive
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [chatState?.messages, chatState?.streamingContent]);

  // Auto-resize textarea
  useEffect(() => {
    if (inputRef.current) {
      inputRef.current.style.height = 'auto';
      inputRef.current.style.height = `${inputRef.current.scrollHeight}px`;
    }
  }, [inputValue]);

  const handleSendMessage = async () => {
    if (!inputValue.trim() || isLoading) return;
    
    const message = inputValue.trim();
    setInputValue('');
    await sendMessage(message);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSendMessage();
    }
  };

  const formatTimestamp = (timestamp: any) => {
    const date = timestamp ? new Date(timestamp) : new Date();
    return date.toLocaleTimeString([], { 
      hour: '2-digit', 
      minute: '2-digit' 
    });
  };

  const renderMessage = (message: Message) => (
    <div key={message.id} className={`message ${message.isUser ? 'user' : 'assistant'} ${message.type}`}>
      <div className="message-content">
        <div className="message-text">
          {message.content}
        </div>
        <div className="message-timestamp">
          {formatTimestamp(message.timestamp)}
        </div>
      </div>
    </div>
  );

  return (
    <div className={`chat-window ${className || ''}`}>
      {/* Header */}
      <div className="chat-header">
        <div className="chat-title">
          <h2>Chat</h2>
          {chatState && (
            <span className="chat-status">
              {chatState.isStreaming ? 'AI is typing...' : 'Ready'}
            </span>
          )}
        </div>
        <div className="chat-actions">
          {chatState?.isStreaming && (
            <button onClick={stopStreaming} className="btn btn-secondary btn-sm">
              Stop
            </button>
          )}
          <button onClick={clearChat} className="btn btn-ghost btn-sm">
            Clear
          </button>
        </div>
      </div>

      {/* Messages */}
      <div className="chat-messages">
        {/* Show loading state */}
        {!chatState && (
          <div className="message system">
            <div className="message-content">
              <div className="message-text">Loading chat...</div>
            </div>
          </div>
        )}
        
        {/* Show empty state */}
        {chatState && (!chatState.messages || chatState.messages.length === 0) && (
          <div className="message system">
            <div className="message-content">
              <div className="message-text">
                👋 Welcome to Loom! Start a conversation to get help with your code.
              </div>
            </div>
          </div>
        )}
        
        {/* Render messages safely */}
        {chatState?.messages?.map(renderMessage)}
        
        {/* Streaming message */}
        {chatState?.isStreaming && chatState.streamingContent && (
          <div className="message assistant streaming">
            <div className="message-content">
              <div className="message-text">
                {chatState.streamingContent}
                <span className="typing-indicator">|</span>
              </div>
            </div>
          </div>
        )}
        
        <div ref={messagesEndRef} />
      </div>

      {/* Input */}
      <div className="chat-input">
        <div className="input-container">
          <textarea
            ref={inputRef}
            value={inputValue}
            onChange={(e) => setInputValue(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Type your message... (Enter to send, Shift+Enter for new line)"
            className="message-input"
            rows={1}
            disabled={isLoading}
          />
          <button
            onClick={handleSendMessage}
            disabled={!inputValue.trim() || isLoading}
            className="send-button btn btn-primary"
          >
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M22 2L11 13" />
              <path d="M22 2L15 22L11 13L2 9L22 2Z" />
            </svg>
          </button>
        </div>
      </div>
    </div>
  );
}