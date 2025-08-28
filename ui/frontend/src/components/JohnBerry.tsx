import React from 'react';

const JohnBerry: React.FC = () => {
    const messages = [
        "John Berry is the sweetest berry ever!",
        "JohnBerry serves up fresh jokes daily.",
        "Bite a JohnBerry for instant happiness.",
    ];

    return (
        <div style={styles.container}>
            <h2>JohnBerry Showcase 🍒</h2>
            <ul style={styles.list}>
                {messages.map((msg, idx) => (
                    <li key={idx} style={styles.item}>{msg}</li>
                ))}
            </ul>
        </div>
    );
};

const styles: { [key: string]: React.CSSProperties } = {
    container: {
        padding: '1rem',
        background: '#ffe0f0',
        borderRadius: '8px',
        margin: '2rem auto',
        maxWidth: '600px',
        textAlign: 'center',
    },
    list: {
        listStyleType: 'square',
        paddingLeft: '1rem',
        textAlign: 'left',
    },
    item: {
        margin: '0.5rem 0',
        color: '#d63384',
    },
};

export default JohnBerry;
