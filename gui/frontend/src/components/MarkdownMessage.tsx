import React, { useEffect, useRef } from 'react';
import { marked } from 'marked';
import hljs from 'highlight.js';

interface MarkdownMessageProps {
    content: string;
    className?: string;
}

export function MarkdownMessage({ content, className }: MarkdownMessageProps) {
    const contentRef = useRef<HTMLDivElement>(null);
    const rendererRef = useRef<any>(null);

    // Set up the renderer only once
    useEffect(() => {
        const renderer = new marked.Renderer();

        // Custom code block rendering with syntax highlighting
        renderer.code = function(token) {
            const code = token.text;
            const language = token.lang;

            if (language && hljs.getLanguage(language)) {
                try {
                    const highlighted = hljs.highlight(code, { language }).value;
                    return `<pre class="hljs-code-block"><code class="hljs language-${language}">${highlighted}</code></pre>`;
                } catch (err) {
                    console.warn('Syntax highlighting failed:', err);
                }
            }
            return `<pre class="hljs-code-block"><code class="hljs">${code}</code></pre>`;
        };

        // Custom inline code rendering
        renderer.codespan = function(token) {
            return `<code class="inline-code">${token.text}</code>`;
        };

        // Custom heading rendering with classes
        renderer.heading = function(token) {
            const level = token.depth;
            const text = this.parser.parseInline(token.tokens);
            return `<h${level} class="markdown-h${level}">${text}</h${level}>`;
        };

        // Custom paragraph rendering
        renderer.paragraph = function(token) {
            const text = this.parser.parseInline(token.tokens);
            return `<p class="markdown-p">${text}</p>`;
        };

        // Custom list rendering
        renderer.list = function(token) {
            const body = this.parser.parse(token.items);
            const tag = token.ordered ? 'ol' : 'ul';
            const className = token.ordered ? 'markdown-ol' : 'markdown-ul';
            return `<${tag} class="${className}">${body}</${tag}>`;
        };

        renderer.listitem = function(token) {
            const text = this.parser.parse(token.tokens);
            return `<li class="markdown-li">${text}</li>`;
        };

        // Custom blockquote rendering
        renderer.blockquote = function(token) {
            const quote = this.parser.parse(token.tokens);
            return `<blockquote class="markdown-blockquote">${quote}</blockquote>`;
        };

        // Custom table rendering
        renderer.table = function(token) {
            let header = '';
            let body = '';

            // Build header
            if (token.header.length) {
                let headerRow = '<tr>';
                for (let i = 0; i < token.header.length; i++) {
                    const cell = this.parser.parseInline(token.header[i].tokens);
                    headerRow += `<th class="markdown-th">${cell}</th>`;
                }
                headerRow += '</tr>';
                header = `<thead>${headerRow}</thead>`;
            }

            // Build body
            if (token.rows.length) {
                let bodyRows = '';
                for (let i = 0; i < token.rows.length; i++) {
                    let row = '<tr>';
                    for (let j = 0; j < token.rows[i].length; j++) {
                        const cell = this.parser.parseInline(token.rows[i][j].tokens);
                        row += `<td class="markdown-td">${cell}</td>`;
                    }
                    row += '</tr>';
                    bodyRows += row;
                }
                body = `<tbody>${bodyRows}</tbody>`;
            }

            return `<div class="table-container"><table class="markdown-table">${header}${body}</table></div>`;
        };

        // Custom link rendering
        renderer.link = function(token) {
            const href = token.href;
            const title = token.title ? ` title="${token.title}"` : '';
            const text = this.parser.parseInline(token.tokens);
            return `<a class="markdown-link" href="${href}" target="_blank" rel="noopener noreferrer"${title}>${text}</a>`;
        };

        // Custom emphasis rendering
        renderer.strong = function(token) {
            const text = this.parser.parseInline(token.tokens);
            return `<strong class="markdown-strong">${text}</strong>`;
        };

        renderer.em = function(token) {
            const text = this.parser.parseInline(token.tokens);
            return `<em class="markdown-em">${text}</em>`;
        };

        // Store the renderer
        rendererRef.current = renderer;

        // Configure marked options
        marked.setOptions({
            renderer,
            gfm: true,
            breaks: true,
        });
    }, []); // Only run once on mount

    // Parse and render content whenever it changes
    useEffect(() => {
        if (contentRef.current && rendererRef.current) {
            // Use requestAnimationFrame to ensure smooth rendering during streaming
            const frameId = requestAnimationFrame(() => {
                if (contentRef.current) {
                    try {
                        const html = marked.parse(content);
                        contentRef.current.innerHTML = typeof html === 'string' ? html : '';
                    } catch (err) {
                        console.error('Markdown parsing failed:', err);
                        contentRef.current.textContent = content; // Fallback to plain text
                    }
                }
            });

            return () => cancelAnimationFrame(frameId);
        }
    }, [content]);

    return (
        <div
            className={`markdown-content ${className || ''}`}
            ref={contentRef}
        />
    );
}
