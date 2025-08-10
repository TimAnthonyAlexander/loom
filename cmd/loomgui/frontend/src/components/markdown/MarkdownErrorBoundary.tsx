import React from 'react';

type Props = { children: React.ReactNode };

type State = { hasError: boolean; error?: Error };

export default class MarkdownErrorBoundary extends React.Component<Props, State> {
    constructor(props: Props) {
        super(props);
        this.state = { hasError: false };
    }

    static getDerivedStateFromError(error: Error) {
        return { hasError: true, error } as State;
    }

    componentDidCatch(error: Error, errorInfo: React.ErrorInfo) {
        // eslint-disable-next-line no-console
        console.error('Markdown rendering error:', error, errorInfo);
    }

    render() {
        if (this.state.hasError) {
            return (
                <div className="markdown-error">
                    <p>Failed to render markdown content:</p>
                    <pre>{this.state.error?.message}</pre>
                    <details>
                        <summary>Raw content</summary>
                        <pre style={{ whiteSpace: 'pre-wrap', fontSize: '0.8em' }}>
                            {typeof this.props.children === 'string' ? this.props.children : 'Content not available'}
                        </pre>
                    </details>
                </div>
            );
        }
        return this.props.children as React.ReactElement;
    }
}


