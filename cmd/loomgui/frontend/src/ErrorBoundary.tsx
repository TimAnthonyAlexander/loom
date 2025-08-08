import { Component, ErrorInfo, ReactNode } from 'react';
import { LogError } from '../wailsjs/runtime/runtime';

interface Props {
    children: ReactNode;
}

interface State {
    hasError: boolean;
    error?: Error;
}

class ErrorBoundary extends Component<Props, State> {
    constructor(props: Props) {
        super(props);
        this.state = { hasError: false };
    }

    static getDerivedStateFromError(error: Error): State {
        return { hasError: true, error };
    }

    componentDidCatch(error: Error, errorInfo: ErrorInfo) {
        const errorMessage = `React Error: ${error.message}\nStack: ${error.stack}\nComponent Stack: ${errorInfo.componentStack}`;

        // Log to console
        console.error('ErrorBoundary caught an error:', error);
        console.error('Error info:', errorInfo);

        // Log to Wails runtime
        try {
            LogError(errorMessage);
        } catch (e) {
            console.error('Failed to log to Wails runtime:', e);
        }
    }

    render() {
        if (this.state.hasError) {
            return (
                <div style={{
                    padding: '20px',
                    margin: '20px',
                    border: '1px solid #ff6b6b',
                    borderRadius: '8px',
                    backgroundColor: '#fff5f5',
                    color: '#d63031'
                }}>
                    <h2>Something went wrong</h2>
                    <p>An error occurred in the application. Check the console for more details.</p>
                    {this.state.error && (
                        <details style={{ marginTop: '10px' }}>
                            <summary>Error Details</summary>
                            <pre style={{
                                fontSize: '12px',
                                overflow: 'auto',
                                backgroundColor: '#f8f9fa',
                                padding: '10px',
                                borderRadius: '4px'
                            }}>
                                {this.state.error.message}
                                {'\n\n'}
                                {this.state.error.stack}
                            </pre>
                        </details>
                    )}
                    <button
                        onClick={() => window.location.reload()}
                        style={{
                            marginTop: '10px',
                            padding: '8px 16px',
                            backgroundColor: '#0984e3',
                            color: 'white',
                            border: 'none',
                            borderRadius: '4px',
                            cursor: 'pointer'
                        }}
                    >
                        Reload Application
                    </button>
                </div>
            );
        }

        return this.props.children;
    }
}

export default ErrorBoundary; 
