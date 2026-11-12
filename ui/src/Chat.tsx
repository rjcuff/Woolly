import { useCallback, useEffect, useRef, useState } from 'react';
import { api, type ChatMessage, type ChatModel, type ChatSummary, type Saved } from './api';

const MODEL_KEY = 'woolly.model';
const LEARN_KEY = 'woolly.learn';

function stored(key: string, fallback: string) {
  try {
    return localStorage.getItem(key) ?? fallback;
  } catch {
    return fallback;
  }
}

function store(key: string, value: string) {
  try {
    localStorage.setItem(key, value);
  } catch {
    // Private windows can refuse storage. The setting just won't stick.
  }
}

type Turn = ChatMessage & { used?: { id: string; content: string }[]; pending?: boolean };

export function ChatView({ onError, onChanged }: { onError: (text: string) => void; onChanged: () => void }) {
  const [models, setModels] = useState<ChatModel[] | null>(null);
  const [model, setModel] = useState('');
  const [offline, setOffline] = useState('');
  const [history, setHistory] = useState<ChatSummary[]>([]);
  const [chatId, setChatId] = useState('');
  const [turns, setTurns] = useState<Turn[]>([]);
  const [text, setText] = useState('');
  const [busy, setBusy] = useState(false);
  const [learn, setLearn] = useState(stored(LEARN_KEY, '1') === '1');
  const abort = useRef<AbortController | null>(null);
  const bottom = useRef<HTMLDivElement>(null);

  const loadHistory = useCallback(() => api.chats().then(setHistory).catch(() => {}), []);

  useEffect(() => {
    api
      .models()
      .then(({ models: list, default: def }) => {
        setModels(list);
        const saved = stored(MODEL_KEY, '');
        const pick = list.find((m) => m.name === saved) ?? list.find((m) => m.name === def) ?? list[0];
        setModel(pick?.name ?? '');
      })
      .catch((e: Error) => {
        setModels([]);
        setOffline(e.message);
      });
    loadHistory();
  }, [loadHistory]);

  useEffect(() => {
    bottom.current?.scrollIntoView({ block: 'end' });
  }, [turns]);

  const open = async (id: string) => {
    abort.current?.abort();
    setChatId(id);
    if (!id) {
      setTurns([]);
      return;
    }
    try {
      const c = await api.chat(id);
      setTurns(c.messages);
      if (c.model && models?.some((m) => m.name === c.model)) setModel(c.model);
    } catch (e) {
      onError((e as Error).message);
    }
  };

  const send = async () => {
    const message = text.trim();
    if (!message || !model || busy) return;
    setText('');
    setBusy(true);
    setTurns((t) => [...t, { role: 'user', content: message }, { role: 'assistant', content: '', pending: true }]);
    const ctrl = new AbortController();
    abort.current = ctrl;
    const patchLast = (f: (t: Turn) => Turn) => setTurns((all) => all.map((t, i) => (i === all.length - 1 ? f(t) : t)));
    const patchUser = (f: (t: Turn) => Turn) => setTurns((all) => all.map((t, i) => (i === all.length - 2 ? f(t) : t)));
    try {
      await api.send(
        { chat_id: chatId || undefined, model, message, learn },
        (e) => {
          if (e.type === 'start') {
            setChatId(e.chat_id);
            patchLast((t) => ({ ...t, used: e.memories ?? [] }));
          } else if (e.type === 'delta') {
            patchLast((t) => ({ ...t, content: t.content + e.content }));
          } else if (e.type === 'saved') {
            patchUser((t) => ({ ...t, saved: e.saved ?? [] }));
            if (e.saved?.length) onChanged();
          } else if (e.type === 'error') {
            onError(e.error);
          }
        },
        ctrl.signal,
      );
    } catch (e) {
      if ((e as Error).name !== 'AbortError') onError((e as Error).message);
    } finally {
      patchLast((t) => ({ ...t, pending: false }));
      setBusy(false);
      loadHistory();
    }
  };

  const undo = async (turn: number, s: Saved) => {
    try {
      await api.forget(s.id);
      if (s.replaces) await api.resolve(s.replaces);
      setTurns((all) =>
        all.map((t, i) => (i === turn ? { ...t, saved: (t.saved ?? []).filter((x) => x.id !== s.id) } : t)),
      );
      onChanged();
    } catch (e) {
      onError((e as Error).message);
    }
  };

  if (models === null) return null;

  if (offline || models.length === 0) {
    return (
      <section className="chat-empty">
        <p>Chat runs on a model in Ollama, on your own computer.</p>
        <ol>
          <li>
            Install Ollama from <strong>ollama.com</strong>
          </li>
          <li>
            Pull a model: <code>ollama pull qwen2.5:3b</code>
          </li>
          <li>Come back to this tab</li>
        </ol>
      </section>
    );
  }

  return (
    <section className="chat">
      <div className="chat-bar">
        <button onClick={() => open('')} disabled={busy}>
          New chat
        </button>
        {history.length > 0 && (
          <select aria-label="Past chats" value={chatId} onChange={(e) => open(e.target.value)} disabled={busy}>
            <option value="">Past chats</option>
            {history.map((c) => (
              <option key={c.id} value={c.id}>
                {c.title || 'Untitled'}
              </option>
            ))}
          </select>
        )}
        <select
          aria-label="Model"
          value={model}
          onChange={(e) => {
            setModel(e.target.value);
            store(MODEL_KEY, e.target.value);
          }}
        >
          {models.map((m) => (
            <option key={m.name} value={m.name}>
              {m.name}
            </option>
          ))}
        </select>
      </div>

      <div className="turns">
        {turns.length === 0 && (
          <p className="hint chat-hint">
            Ask anything. Woolly adds what it knows about you to each message, and saves new facts you mention.
          </p>
        )}
        {turns.map((t, i) => (
          <div key={i} className={`turn ${t.role}`}>
            <div className="bubble">{t.content || (t.pending ? '…' : '')}</div>
            {t.role === 'assistant' && t.used && t.used.length > 0 && (
              <details className="used">
                <summary>Used {t.used.length === 1 ? '1 memory' : `${t.used.length} memories`}</summary>
                <ul>
                  {t.used.map((m) => (
                    <li key={m.id}>{m.content}</li>
                  ))}
                </ul>
              </details>
            )}
            {t.role === 'user' &&
              (t.saved ?? []).map((s) => (
                <div key={s.id} className="saved">
                  <span>
                    {s.action === 'update' ? 'Updated' : s.action === 'conflict' ? 'Saved for review' : 'Saved'}: {s.fact}
                  </span>
                  <button className="link" onClick={() => undo(i, s)}>
                    Undo
                  </button>
                </div>
              ))}
          </div>
        ))}
        <div ref={bottom} />
      </div>

      <form
        className="composer"
        onSubmit={(e) => {
          e.preventDefault();
          send();
        }}
      >
        <textarea
          aria-label="Message"
          placeholder="Message"
          rows={1}
          value={text}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) {
              e.preventDefault();
              send();
            }
          }}
        />
        {busy ? (
          <button type="button" onClick={() => abort.current?.abort()}>
            Stop
          </button>
        ) : (
          <button className="primary" type="submit" disabled={!text.trim()}>
            Send
          </button>
        )}
      </form>
      <label className="learn-toggle">
        <input
          type="checkbox"
          checked={learn}
          onChange={(e) => {
            setLearn(e.target.checked);
            store(LEARN_KEY, e.target.checked ? '1' : '0');
          }}
        />
        Save new facts from this chat
      </label>
    </section>
  );
}
