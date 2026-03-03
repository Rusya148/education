import { useState, useEffect } from 'react';

// --- Components ---

const Header = () => (
    <header className="header">
        <div className="logo" onClick={() => console.log('Logo clicked')}>
            minion<div className="eye"></div>ns
        </div>
        <nav className="nav">
            {['Главная', 'Каталог', 'Доставка и оплата', 'О нас', 'Контакты'].map((item) => (
                <a key={item} href="#" className="nav-link" onClick={(e) => { e.preventDefault(); console.log(`${item} clicked`); }}>
                    {item}
                </a>
            ))}
        </nav>
        <button className="btn-to-shop" onClick={() => console.log('To shop clicked')}>
            К покупкам
        </button>
    </header>
);

const Hero = ({ onOpenForm }) => (
    <section className="hero">
        <div className="hero-content">
            <h1>Оформите нашу карту.</h1>
            <p>и получите 500 рублей в качестве бонуса!</p>
            <button className="btn-apply" onClick={onOpenForm}>
                Оформить
            </button>
        </div>
    </section>
);

const ApplyModal = ({ onClose }) => {
    const [formData, setFormData] = useState({ name: '', phone: '', email: '' });
    const [status, setStatus] = useState({ loading: false, success: false, error: '' });

    const handleChange = (e) => {
        setFormData({ ...formData, [e.target.name]: e.target.value });
    };

    const handleSubmit = async (e) => {
        e.preventDefault();
        setStatus({ loading: true, success: false, error: '' });

        const apiUrl = window._env_?.API_URL || 'http://localhost:8080';

        try {
            const response = await fetch(`${apiUrl}/apply`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(formData),
            });

            if (!response.ok) throw new Error('Ошибка сервера');

            setStatus({ loading: false, success: true, error: '' });
            setTimeout(() => {
                onClose();
                setFormData({ name: '', phone: '', email: '' });
            }, 2000);
        } catch (err) {
            setStatus({ loading: false, success: false, error: 'Не удалось отправить заявку' });
        }
    };

    return (
        <div className="form-overlay" onClick={onClose}>
            <div className="apply-modal" onClick={(e) => e.stopPropagation()}>
                <button className="btn-close" onClick={onClose}>&times;</button>
                <h2>Заявка на карту</h2>
                {status.success ? (
                    <div className="msg-success">Успешно! Мы свяжемся с вами.</div>
                ) : (
                    <form onSubmit={handleSubmit}>
                        <input name="name" placeholder="Имя" value={formData.name} onChange={handleChange} required />
                        <input name="phone" placeholder="Телефон" type="tel" value={formData.phone} onChange={handleChange} required />
                        <input name="email" placeholder="Email" type="email" value={formData.email} onChange={handleChange} required />
                        <button className="btn-submit" disabled={status.loading}>
                            {status.loading ? 'Отправка...' : 'Отправить'}
                        </button>
                        {status.error && <div className="msg-error">{status.error}</div>}
                    </form>
                )}
            </div>
        </div>
    );
};

// --- Main App ---

function App() {
    const [isFormOpen, setIsFormOpen] = useState(false);

    useEffect(() => {
        console.log('App initialized with API:', window._env_?.API_URL);
    }, []);

    return (
        <div className="app-container">
            <Header />
            <Hero onOpenForm={() => setIsFormOpen(true)} />
            {isFormOpen && <ApplyModal onClose={() => setIsFormOpen(false)} />}
            <div className="chat-icon" onClick={() => console.log('Chat clicked')}></div>
        </div>
    );
}

export default App;
