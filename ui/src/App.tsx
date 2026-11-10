import { useCallback, useEffect, useMemo, useState } from 'react';
import { api, type Memory, splitTags } from './api';

type View = 'current' | 'conflicts' | 'all' | 'profile';

const views: { id: View; label: string }[] = [
  { id: 'current', label: 'Memories' },
  { id: 'conflicts', label: 'Conflicts' },
  { id: 'all', label: 'History' },
  { id: 'profile', label: 'Profile' },
];

type Notice = { kind: 'ok' | 'error'; text: string } | null;

export function App() {
  const [view, setView] = useState<View>('current');
  const [query, setQuery] = useState('');
  const [items, setItems] = useState<Memory[]>([]);
  const [conflictCount, setConflictCount] = useState(0);
  const [byId, setById] = useState<Map<string, Memory>>(new Map());
  const [profileText, setProfileText] = useState('');
  const [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState<Notice>(null);

  const flash = useCallback((n: Notice) => {
    setNotice(n);
    if (n?.kind === 'ok') window.setTimeout(() => setNotice((cur) => (cur === n ? null : cur)), 2500);
  }, []);

  const load = useCallback(async () => {
    try {
      const [all, conflicts] = await Promise.all([api.list(true), api.conflicts()]);
      setById(new Map(all.map((m) => [m.id, m])));
      setConflictCount(conflicts.length);
      if (view === 'profile') {
        setProfileText(await api.profile());
      } else if (view === 'conflicts') {
        setItems(conflicts);
      } else if (query.trim()) {
        setItems(await api.recall(query, view === 'all'));
      } else {
        setItems(view === 'all' ? all : all.filter((m) => m.status === 'active'));
      }
    } catch (e) {
      flash({ kind: 'error', text: (e as Error).message });
    } finally {
      setLoading(false);
    }
  }, [view, query, flash]);

  useEffect(() => {
    const t = window.setTimeout(load, query ? 200 : 0);
    return () => window.clearTimeout(t);
  }, [load, query]);

  const run = useCallback(
    async (action: () => Promise<unknown>, done: string) => {
      try {
        await action();
        flash({ kind: 'ok', text: done });
        await load();
        return true;
      } catch (e) {
        flash({ kind: 'error', text: (e as Error).message });
        return false;
      }
    },
    [flash, load],
  );

  return (
    <div className="app">
      <header className="top">
        <span className="wordmark">Woolly</span>
        <nav className="tabs" aria-label="Views">
          {views.map((v) => (
            <button
              key={v.id}
              className={view === v.id ? 'tab active' : 'tab'}
              aria-current={view === v.id ? 'page' : undefined}
              onClick={() => setView(v.id)}
            >
              {v.label}
              {v.id === 'conflicts' && conflictCount > 0 && <span className="badge">{conflictCount}</span>}
            </button>
          ))}
        </nav>
      </header>

      <main className="main">
        {notice && (
          <div className={`notice ${notice.kind}`} role={notice.kind === 'error' ? 'alert' : 'status'}>
            <span>{notice.text}</span>
            <button className="link" onClick={() => setNotice(null)} aria-label="Dismiss">
              ×
            </button>
          </div>
        )}

        {view === 'profile' ? (
          <ProfileView text={profileText} onCopied={() => flash({ kind: 'ok', text: 'Copied' })} />
        ) : (
          <>
            {view === 'current' && <AddForm onAdd={(c, k, t) => run(() => api.remember(c, k, t), 'Saved')} />}
            {view !== 'conflicts' && (
              <input
                id="search"
                className="search"
                type="search"
                placeholder="Search memories"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
              />
            )}
            {view === 'conflicts' && items.length > 0 && (
              <p className="hint">
                These disagree with something you told Woolly directly. Keep the one that's right.
              </p>
            )}
            {loading ? null : items.length === 0 ? (
              <Empty view={view} searching={!!query.trim()} />
            ) : (
              <ul className="list">
                {items.map((m) => (
                  <MemoryRow
                    key={m.id}
                    memory={m}
                    other={m.conflicts_with ? byId.get(m.conflicts_with) : undefined}
                    replacedBy={m.superseded_by ? byId.get(m.superseded_by) : undefined}
                    onSave={(patch) => run(() => api.update(m.id, patch), 'Saved')}
                    onForget={() => run(() => api.forget(m.id), 'Forgotten')}
                    onKeep={(id) => run(() => api.resolve(id), 'Updated')}
                  />
                ))}
              </ul>
            )}
          </>
        )}
      </main>
    </div>
  );
}

function Empty({ view, searching }: { view: View; searching: boolean }) {
  const text = searching
    ? 'No memories match that search.'
    : view === 'conflicts'
      ? 'No conflicts. Everything agrees.'
      : 'No memories yet. Add one above, or connect an AI app.';
  return <p className="empty">{text}</p>;
}

function AddForm({ onAdd }: { onAdd: (content: string, key: string, tags: string[]) => Promise<boolean> }) {
  const [content, setContent] = useState('');
  const [key, setKey] = useState('');
  const [tags, setTags] = useState('');
  const [more, setMore] = useState(false);

  return (
    <form
      className="add"
      onSubmit={async (e) => {
        e.preventDefault();
        if (!content.trim()) return;
        const sent = { content, key, tags };
        if (await onAdd(content, key, splitTags(tags))) {
          // Only clear what hasn't been changed since saving.
          setContent((c) => (c === sent.content ? '' : c));
          setKey((k) => (k === sent.key ? '' : k));
          setTags((t) => (t === sent.tags ? '' : t));
        }
      }}
    >
      <div className="add-row">
        <input
          id="new-memory"
          aria-label="New memory"
          placeholder="Remember something, like “I prefer short answers”"
          value={content}
          onChange={(e) => setContent(e.target.value)}
        />
        <button className="primary" type="submit" disabled={!content.trim()}>
          Save
        </button>
      </div>
      {more ? (
        <div className="add-row">
          <input id="new-key" aria-label="Key" placeholder="Key, like user.location (optional)" value={key} onChange={(e) => setKey(e.target.value)} />
          <input id="new-tags" aria-label="Tags" placeholder="Tags, comma separated (optional)" value={tags} onChange={(e) => setTags(e.target.value)} />
        </div>
      ) : (
        <button type="button" className="link small" onClick={() => setMore(true)}>
          Add a key or tags
        </button>
      )}
    </form>
  );
}

function MemoryRow({
  memory: m,
  other,
  replacedBy,
  onSave,
  onForget,
  onKeep,
}: {
  memory: Memory;
  other?: Memory;
  replacedBy?: Memory;
  onSave: (patch: { content: string; key: string; tags: string[] }) => Promise<boolean>;
  onForget: () => Promise<boolean>;
  onKeep: (id: string) => Promise<boolean>;
}) {
  const [editing, setEditing] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [content, setContent] = useState(m.content);
  const [key, setKey] = useState(m.key ?? '');
  const [tags, setTags] = useState((m.tags ?? []).join(', '));

  const date = useMemo(
    () => new Date(m.updated_at).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' }),
    [m.updated_at],
  );

  if (editing) {
    return (
      <li className="row editing">
        <textarea aria-label="Edit memory" value={content} onChange={(e) => setContent(e.target.value)} rows={2} />
        <div className="add-row">
          <input aria-label="Edit key" placeholder="Key (optional)" value={key} onChange={(e) => setKey(e.target.value)} />
          <input aria-label="Edit tags" placeholder="Tags (optional)" value={tags} onChange={(e) => setTags(e.target.value)} />
        </div>
        <div className="actions">
          <button
            className="primary"
            onClick={async () => {
              if (await onSave({ content, key, tags: splitTags(tags) })) setEditing(false);
            }}
          >
            Save
          </button>
          <button onClick={() => setEditing(false)}>Cancel</button>
        </div>
      </li>
    );
  }

  return (
    <li className={`row ${m.status}`}>
      <div className="content">{m.content}</div>
      <div className="meta">
        {m.key && <span className="chip key">{m.key}</span>}
        {(m.tags ?? []).map((t) => (
          <span key={t} className="chip">
            {t}
          </span>
        ))}
        <span>{sourceLabel(m.source)}</span>
        <span>{date}</span>
        {m.status === 'superseded' && <span className="status">Replaced{replacedBy ? ` by “${replacedBy.content}”` : ''}</span>}
      </div>

      {m.status === 'conflict' && other && (
        <div className="versus">
          <div className="versus-label">Current fact</div>
          <div>{other.content}</div>
        </div>
      )}

      <div className="actions">
        {m.status === 'conflict' ? (
          <>
            <button className="primary" onClick={() => onKeep(m.id)}>
              Keep this
            </button>
            {other && <button onClick={() => onKeep(other.id)}>Keep current fact</button>}
          </>
        ) : m.status === 'superseded' ? (
          <button onClick={() => onKeep(m.id)}>Make current</button>
        ) : (
          <button onClick={() => setEditing(true)}>Edit</button>
        )}
        {confirming ? (
          <>
            <button className="danger" onClick={() => onForget()}>
              Delete for good
            </button>
            <button onClick={() => setConfirming(false)}>Cancel</button>
          </>
        ) : (
          <button className="quiet" onClick={() => setConfirming(true)}>
            Forget
          </button>
        )}
      </div>
    </li>
  );
}

function sourceLabel(s: Memory['source']) {
  return s === 'user' ? 'You said' : s === 'import' ? 'Imported' : 'AI guessed';
}

function ProfileView({ text, onCopied }: { text: string; onCopied: () => void }) {
  return (
    <section className="profile">
      <p className="hint">
        A summary of what Woolly knows about you. Paste it into any AI's custom instructions.
      </p>
      <pre>{text}</pre>
      <button
        className="primary"
        onClick={async () => {
          try {
            await navigator.clipboard.writeText(text);
            onCopied();
          } catch {
            const sel = window.getSelection();
            const pre = document.querySelector('.profile pre');
            if (sel && pre) sel.selectAllChildren(pre);
          }
        }}
      >
        Copy profile
      </button>
    </section>
  );
}
