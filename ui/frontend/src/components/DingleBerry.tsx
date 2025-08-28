import React, { useState } from 'react';
import JohnBerry from './JohnBerry';

interface BerryFacts {
    [key: string]: string;
}

const facts: BerryFacts = {
    'Ruby Dingle': 'Legend says Ruby Dingle glows under the full moon, guiding lost travelers.',
    'Sapphire Dingle': 'Sapphire Dingle is rumored to heal heartaches with a single touch.',
    'Emerald Dingle': 'Emerald Dingle grants visions of the future to those who taste it.',
    'Golden Dingle': 'Golden Dingle brings boundless luck and prosperity to its bearer.'
};

const DingleBerryPage: React.FC = () => {
    const berries = Object.keys(facts);
    const [selectedFact, setSelectedFact] = useState<string | null>(null);

    const showFact = (berry: string) => {
        setSelectedFact(facts[berry]);
    };

    return (
        <div style={styles.page}>
            <header style={styles.header}>
                <h2>DingleBerry World</h2>
            </header>
            <section style={styles.hero}>
                <h1>Explore Mystical DingleBerries</h1>
                <p>Discover the secrets hidden within each shimmering berry.</p>
            </section>
            <main style={styles.grid}>
                {berries.map((berry) => (
                    <div key={berry} style={styles.card}>
                        <div style={{ fontSize: '3rem' }}>🍇</div>
                        <h3>{berry}</h3>
                        <button onClick={() => showFact(berry)} style={styles.button}>
                            Reveal Secret
                        </button>
                    </div>
                ))}
            </main>
            <JohnBerry />
            {selectedFact && (
                <section style={styles.factSection}>
                    <h4>Secret Revealed:</h4>
                    <p>{selectedFact}</p>
                </section>
            )}
        </div>
    );
};

const styles: { [key: string]: React.CSSProperties } = {
    page: {
        minHeight: '100vh',
        margin: 0,
        fontFamily: 'Arial, sans-serif',
        background: '#fafafa',
        color: '#333'
    },
    header: {
        background: '#4b0082',
        color: '#fff',
        padding: '1rem 2rem',
        textAlign: 'center'
    },
    hero: {
        padding: '4rem 2rem',
        textAlign: 'center',
        background: 'linear-gradient(135deg, #72edf2 10%, #5151e5 100%)',
        color: '#fff'
    },
    grid: {
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))',
        gap: '1rem',
        padding: '2rem'
    },
    card: {
        background: '#fff',
        borderRadius: '8px',
        padding: '1.5rem',
        boxShadow: '0 2px 8px rgba(0,0,0,0.1)',
        textAlign: 'center'
    },
    button: {
        marginTop: '1rem',
        padding: '0.5rem 1rem',
        border: 'none',
        borderRadius: '4px',
        background: '#4b0082',
        color: '#fff',
        cursor: 'pointer'
    },
    factSection: {
        margin: '2rem auto',
        maxWidth: '600px',
        padding: '1rem 2rem',
        background: '#fff',
        borderRadius: '8px',
        boxShadow: '0 2px 8px rgba(0,0,0,0.1)',
        textAlign: 'center'
    },
    footer: {
        background: '#333',
        color: '#fff',
        textAlign: 'center',
        padding: '1rem 0',
        marginTop: '2rem'
    }
};

export default DingleBerryPage;
