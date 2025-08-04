import React from 'react';
import ReactMarkdown from 'react-markdown';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { oneDark } from 'react-syntax-highlighter/dist/esm/styles/prism';
import type { Components } from 'react-markdown';

interface MarkdownMessageProps {
  content: string;
  className?: string;
}

export function MarkdownMessage({ content, className }: MarkdownMessageProps) {
  const components: Components = {
    code(props) {
      const { children, className } = props;
      const match = /language-(\w+)/.exec(className || '');
      
      return match ? (
        <SyntaxHighlighter
          PreTag="div"
          children={String(children).replace(/\n$/, '')}
          language={match[1]}
          style={oneDark}
          customStyle={{
            margin: '1em 0',
            borderRadius: '8px',
            fontSize: '0.9em',
          }}
        />
      ) : (
        <code className={`${className} inline-code`}>
          {children}
        </code>
      );
    },
    // Custom styling for other elements
    h1: ({ children }) => <h1 className="markdown-h1">{children}</h1>,
    h2: ({ children }) => <h2 className="markdown-h2">{children}</h2>,
    h3: ({ children }) => <h3 className="markdown-h3">{children}</h3>,
    h4: ({ children }) => <h4 className="markdown-h4">{children}</h4>,
    h5: ({ children }) => <h5 className="markdown-h5">{children}</h5>,
    h6: ({ children }) => <h6 className="markdown-h6">{children}</h6>,
    p: ({ children }) => <p className="markdown-p">{children}</p>,
    ul: ({ children }) => <ul className="markdown-ul">{children}</ul>,
    ol: ({ children }) => <ol className="markdown-ol">{children}</ol>,
    li: ({ children }) => <li className="markdown-li">{children}</li>,
    blockquote: ({ children }) => <blockquote className="markdown-blockquote">{children}</blockquote>,
    table: ({ children }) => (
      <div className="table-container">
        <table className="markdown-table">{children}</table>
      </div>
    ),
    th: ({ children }) => <th className="markdown-th">{children}</th>,
    td: ({ children }) => <td className="markdown-td">{children}</td>,
    a: ({ children, href }) => (
      <a className="markdown-link" href={href} target="_blank" rel="noopener noreferrer">
        {children}
      </a>
    ),
    strong: ({ children }) => <strong className="markdown-strong">{children}</strong>,
    em: ({ children }) => <em className="markdown-em">{children}</em>,
  };

  return (
    <div className={`markdown-content ${className || ''}`}>
      <ReactMarkdown
        components={components}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
}