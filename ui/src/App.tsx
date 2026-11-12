import { useCallback, useEffect, useMemo, useState } from 'react';
import { type Activity, api, type Memory, splitTags } from './api';

type View = 'current' | 'conflicts' | 'all' | 'trash' | 'activity' | 'profile';

const views: { id: View; label: string }[] = [
  { id: 'current', label: 'Memories' },
  { id: 'conflicts', label: 'Conflicts' },
  { id: 'all', label: 'History' },
  { id: 'trash', label: 'Trash' },
  { id: 'activity', label: 'Activity' },
  { id: 'profile', label: 'Profile' },
];

// ?view=activity opens a tab directly.
function initialView(): View {
  const v = new URLSearchParams(window.location.search).get('view');
  return views.some((x) => x.id === v) ? (v as View) : 'current';
}

type Notice = { kind: 'ok' | 'error'; text: string; undo?: () => void } | null;

export function App() {
  const [view, setView] = useState<View>(initialView);
  const [query, setQuery] = useState('');
  const [items, setItems] = useState<Memory[]>([]);
  const [conflictCount, setConflictCount] = useState(0);
  const [byId, setById] = useState<Map<string, Memory>>(new Map());
  const [profileText, setProfileText] = useState('');
  const [activity, setActivity] = useState<Activity[]>([]);
  const [trashById, setTrashById] = useState<Map<string, Memory>>(new Map());
  const [loading, setLoading] = useState(true);
  const [project, setProject] = useState('');
  const [projects, setProjects] = useState<string[]>([]);
  const [notice, setNotice] = useState<Notice>(null);

  const flash = useCallback((n: Notice) => {
    setNotice(n);
    if (n?.kind === 'ok') window.setTimeout(() => setNotice((cur) => (cur === n ? null : cur)), n.undo ? 6000 : 2500);
  }, []);

  const load = useCallback(async () => {
    try {
      const [all, conflicts] = await Promise.all([api.list(true), api.conflicts()]);
      setById(new Map(all.map((m) => [m.id, m])));
      setProjects([...new Set(all.map((m) => m.project).filter((p): p is string => !!p))].sort());
      setConflictCount(conflicts.length);
      if (view === 'profile') {
        setProfileText(await api.profile());
      } else if (view === 'activity') {
        const [log, trash] = await Promise.all([api.activity(), api.trash()]);
        setActivity(log);
        setTrashById(new Map(trash.map((m) => [m.id, m])));
      } else if (view === 'trash') {
        setItems((await api.trash()).map((m) => ({ ...m, inTrash: true })));
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
        <a className="wordmark" href="/app" title="Back to chat">
          Woolly
        </a>
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
            {notice.undo && (
              <button className="link undo" onClick={notice.undo}>
                Undo
              </button>
            )}
            <button className="link" onClick={() => setNotice(null)} aria-label="Dismiss">
              ×
            </button>
          </div>
        )}

        {view === 'profile' ? (
          <ProfileView text={profileText} onCopied={() => flash({ kind: 'ok', text: 'Copied' })} />
        ) : view === 'activity' ? (
          loading ? null : <ActivityView entries={activity} find={(id) => byId.get(id) ?? trashById.get(id)} />
        ) : (
          <>
            {view === 'current' && <AddForm onAdd={(c, k, t) => run(() => api.remember(c, k, t), 'Saved')} />}
            {view === 'trash' && items.length > 0 && <TrashBar onEmpty={() => run(() => api.emptyTrash(), 'Trash emptied')} />}
            {view !== 'conflicts' && view !== 'trash' && (
              <div className="search-row">
                <input
                  id="search"
                  className="search"
                  type="search"
                  placeholder="Search memories"
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                />
                {projects.length > 0 && (
                  <select aria-label="Project" value={project} onChange={(e) => setProject(e.target.value)}>
                    <option value="">All projects</option>
                    <option value="-">About you</option>
                    {projects.map((p) => (
                      <option key={p} value={p}>
                        {p}
                      </option>
                    ))}
                  </select>
                )}
              </div>
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
                {items.filter((m) => !project || (project === '-' ? !m.project : m.project === project)).map((m) => (
                  <MemoryRow
                    key={m.id}
                    memory={m}
                    other={m.conflicts_with ? byId.get(m.conflicts_with) : undefined}
                    replacedBy={m.superseded_by ? byId.get(m.superseded_by) : undefined}
                    onSave={(patch) => run(() => api.update(m.id, patch), 'Saved')}
                    onForget={async () => {
                      try {
                        await api.forget(m.id);
                        flash({
                          kind: 'ok',
                          text: 'Moved to trash',
                          undo: () => run(() => api.restore(m.id), 'Restored'),
                        });
                        await load();
                      } catch (e) {
                        flash({ kind: 'error', text: (e as Error).message });
                      }
                    }}
                    onRestore={() => run(() => api.restore(m.id), 'Restored')}
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
      : view === 'trash'
        ? 'The trash is empty.'
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
  onRestore,
  onKeep,
}: {
  memory: Memory;
  other?: Memory;
  replacedBy?: Memory;
  onSave: (patch: { content: string; key: string; tags: string[]; project: string }) => Promise<boolean>;
  onForget: () => Promise<void>;
  onRestore: () => Promise<boolean>;
  onKeep: (id: string) => Promise<boolean>;
}) {
  const [editing, setEditing] = useState(false);
  const inTrash = !!m.inTrash;
  const [content, setContent] = useState(m.content);
  const [key, setKey] = useState(m.key ?? '');
  const [tags, setTags] = useState((m.tags ?? []).join(', '));
  const [proj, setProj] = useState(m.project ?? '');

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
          <input aria-label="Edit project" placeholder="Project (empty means about you)" value={proj} onChange={(e) => setProj(e.target.value)} />
        </div>
        <div className="actions">
          <button
            className="primary"
            onClick={async () => {
              if (await onSave({ content, key, tags: splitTags(tags), project: proj })) setEditing(false);
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
        {m.project && <span className="chip project">{m.project}</span>}
        {m.key && <span className="chip key">{m.key}</span>}
        {(m.tags ?? []).map((t) => (
          <span key={t} className="chip">
            {t}
          </span>
        ))}
        <span>{sourceLabel(m.source)}</span>
        {m.via && <span>via {m.via}</span>}
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
        {inTrash ? (
          <button className="primary" onClick={() => onRestore()}>
            Restore
          </button>
        ) : (
          <>
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
            <button className="quiet" onClick={() => onForget()}>
              Forget
            </button>
          </>
        )}
      </div>
    </li>
  );
}

function TrashBar({ onEmpty }: { onEmpty: () => Promise<boolean> }) {
  const [confirming, setConfirming] = useState(false);
  return (
    <div className="trash-bar">
      <p className="hint">Forgotten memories stay here until you empty the trash.</p>
      {confirming ? (
        <div className="actions">
          <button className="danger" onClick={() => onEmpty()}>
            Delete all for good
          </button>
          <button onClick={() => setConfirming(false)}>Cancel</button>
        </div>
      ) : (
        <button onClick={() => setConfirming(true)}>Empty trash</button>
      )}
    </div>
  );
}

function sourceLabel(s: Memory['source']) {
  return s === 'user' ? 'You said' : s === 'import' ? 'Imported' : 'AI guessed';
}

const verbs: Record<string, string> = {
  remember: 'Saved',
  recall: 'Searched',
  profile: 'Read your profile',
  list: 'Listed all memories',
  context: 'Loaded your profile at startup',
  update: 'Edited',
  forget: 'Deleted',
  restore: 'Restored',
  resolve: 'Settled a conflict, keeping',
  empty_trash: 'Emptied the trash',
  import: 'Imported',
  learn: 'Learned',
  sync: 'Synced',
};

function describeEntry(e: Activity, find: (id: string) => Memory | undefined) {
  const verb = verbs[e.action] ?? e.action;
  const first = e.ids?.[0];
  const subject = first ? (find(first)?.content ?? 'a memory that was deleted') : '';
  switch (e.action) {
    case 'remember':
    case 'update':
    case 'forget':
    case 'restore':
    case 'resolve':
      return `${verb} “${subject}”`;
    case 'recall':
      return e.query ? `${verb} for “${e.query}”, got ${e.count ?? 0}` : `Read ${e.count ?? 0} memories`;
    case 'import':
    case 'learn':
    case 'sync':
      return `${verb} ${e.count ?? 0} memories`;
    case 'empty_trash':
      return `${verb} (${e.count ?? 0})`;
    case 'context':
      return e.ids?.length ? `${verb}, plus ${e.ids.length} about this project` : verb;
  }
  return verb;
}

function ActivityView({ entries, find }: { entries: Activity[]; find: (id: string) => Memory | undefined }) {
  const [app, setApp] = useState('');
  const apps = useMemo(() => [...new Set(entries.map((e) => e.app))].sort(), [entries]);
  const shown = app ? entries.filter((e) => e.app === app) : entries;
  if (entries.length === 0) {
    return <p className="empty">No activity yet. When an app saves, reads or deletes a memory, it shows up here.</p>;
  }
  return (
    <section className="activity">
      <div className="activity-bar">
        <p className="hint">What each app saved, read and deleted. This log stays on your computer.</p>
        {apps.length > 1 && (
          <select aria-label="App" value={app} onChange={(e) => setApp(e.target.value)}>
            <option value="">All apps</option>
            {apps.map((a) => (
              <option key={a} value={a}>
                {a}
              </option>
            ))}
          </select>
        )}
      </div>
      <ul className="list">
        {shown.map((e, i) => (
          <li key={`${e.time}-${i}`} className="row log">
            <div className="content">{describeEntry(e, find)}</div>
            <div className="meta">
              <span className="chip">{e.app}</span>
              <span>
                {new Date(e.time).toLocaleString('en-US', { month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' })}
              </span>
            </div>
          </li>
        ))}
      </ul>
    </section>
  );
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
