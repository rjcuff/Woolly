import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { App } from './App';
import { ChatApp } from './ChatApp';
import './styles.css';

// /app is the chat. Everything else is the memory manager.
const chat = window.location.pathname.replace(/\/+$/, '') === '/app';

createRoot(document.getElementById('root')!).render(<StrictMode>{chat ? <ChatApp /> : <App />}</StrictMode>);
