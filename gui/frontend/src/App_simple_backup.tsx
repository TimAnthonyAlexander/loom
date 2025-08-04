import React from 'react';
import './styles/globals.css';

function App() {
  return (
    <div style={{ padding: '20px', fontFamily: 'Arial, sans-serif' }}>
      <h1 style={{ color: '#333' }}>🎉 Loom GUI Test</h1>
      <p style={{ color: '#666' }}>If you can see this, the React app is working!</p>
      <div style={{ 
        background: '#f0f0f0', 
        padding: '20px', 
        borderRadius: '8px',
        margin: '20px 0'
      }}>
        <h2>Test Component</h2>
        <button onClick={() => alert('Button clicked!')}>Click me!</button>
      </div>
    </div>
  );
}

export default App;