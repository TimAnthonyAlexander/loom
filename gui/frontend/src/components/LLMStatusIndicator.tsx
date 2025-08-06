import React from 'react';
import './LLMStatusIndicator.css';

interface LLMStatusIndicatorProps {
  isStreaming: boolean;
  isLoading: boolean;
}

export function LLMStatusIndicator({ isStreaming, isLoading }: LLMStatusIndicatorProps) {
  let status = 'idle';
  let statusText = 'Ready';
  
  if (isLoading && !isStreaming) {
    status = 'thinking';
    statusText = 'Thinking...';
  } else if (isStreaming) {
    status = 'streaming';
    statusText = 'Responding...';
  }
  
  return (
    <div className={`llm-status-indicator ${status}`}>
      {status !== 'idle' && <div className="pulse-dot" />}
      <span className="status-text">{statusText}</span>
    </div>
  );
}