import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { api, type Memory } from '../api';
import { Empty } from './Library';
import { AppLogo } from './logos';
import { ago, I, Icon, Skeleton, type Toast } from './ui';

/** Every current memory: add, search, edit in place and forget. */
export function Memories({ onToast, onChanged, refresh }: { onToast: (t: Toast) => void; onChanged: () => void; refresh: number }) {
  const [items, setItems] = useState<Memory[] | null>(null);
  const [all, setAll] = useState<Memory[]>([]);
  const [query, setQuery] = useState('');
  const [filter, setFilter] = useState('');
  const [draft, setDraft] = useState('');
  const [editing, setEditing] = useState('');
  const [edit, setEdit] = useState('');
  const editRef = useRef<HTMLTextAreaElement>(null);

  const load = useCallback(async () => {
    try {
      const list = await api.list(false);
      setAll(list);
      setItems(query.trim() ? await api.recall(query, false) : list);
    } catch (e) {
      onToast({ text: (e as Error).message });
    }
  }, [query, onToast]);

  // biome-ignore lint/correctness/useExhaustiveDependencies: refresh is a signal to reload.
  useEffect(() => {
    const t = window.setTimeout(load, query ? 200 : 0);
    return () => window.clearTimeout(t);
  }, [load, refresh]);

  useEffect(() => {
    if (editing) editRef.current?.focus();
  }, [editing]);

  const projects = useMemo(() => [...new Set(all.map((m) => m.project).filter((p): p is string => !!p))].sort(), [all]);
  const shown = (items ?? []).filter((m) => !filter || (filter === '-' ? !m.project : m.project === filter));

  const add = async () => {
    const text = draft.trim();
    if (!text) return;
    try {
      await api.remember(text, '', []);
      setDraft('');
      load();
      onChanged();
    } catch (e) {
      onToast({ text: (e as Error).message });
    }
  };

  const save = async (m: Memory) => {
    const text = edit.trim();
    setEditing('');
    if (!text || text === m.content) return;
    try {
      await api.update(m.id, { content: text });
      load();
      onChanged();
    } catch (e) {
      onToast({ text: (e as Error).message });
    }
  };

  const forget = async (m: Memory) => {
    try {
      await api.forget(m.id);
      setItems((list) => (list ?? []).filter((x) => x.id !== m.id));
      onChanged();
      onToast({
        text: 'Forgotten',
        undo: async () => {
          await api.restore(m.id);
          load();
          onChanged();
        },
      });
    } catch (e) {
      onToast({ text: (e as Error).message });
    }
  };

  return (
    <div className="ca-page narrow">
      <div className="ca-title-row">
        <h1 className="ca-title big">Memories</h1>
        {items && <span className="ca-title-aside dim">{all.length}</span>}
      </div>

      <form
        className="add-box"
        onSubmit={(e) => {
          e.preventDefault();
          add();
        }}
      >
        <Icon d={I.plus} size={18} />
        <input aria-label="New memory" placeholder="Add a memory, like “I prefer short answers”" value={draft} onChange={(e) => setDraft(e.target.value)} />
        {draft.trim() && (
          <button type="submit" className="btn primary">
            Save
          </button>
        )}
      </form>

      <div className="mem-tools">
        <label className="search-box">
          <Icon d={I.search} size={16} />
          <input type="search" aria-label="Search memories" placeholder="Search" value={query} onChange={(e) => setQuery(e.target.value)} />
        </label>
        {projects.length > 0 && (
          <div className="chips" role="group" aria-label="Filter">
            {[
              ['', 'All'],
              ['-', 'About you'],
              ...projects.map((p) => [p, p]),
            ].map(([value, label]) => (
              <button key={value} className={`chip${filter === value ? ' on' : ''}`} aria-pressed={filter === value} onClick={() => setFilter(value)}>
                {label}
              </button>
            ))}
          </div>
        )}
      </div>

      {items === null && (
        <ul className="mem-list">
          {[0, 1, 2, 3, 4].map((i) => (
            <li key={i} className="mem-row">
              <Skeleton w={32} h={32} r={8} />
              <div className="mem-body">
                <Skeleton w={`${80 - i * 9}%`} h={14} />
                <Skeleton w={110} h={11} className="skel-gap" />
              </div>
            </li>
          ))}
        </ul>
      )}

      {items !== null && shown.length === 0 && (
        <Empty
          icon={query || filter ? I.search : I.memory}
          title={query || filter ? 'No matches' : 'No memories yet'}
          text={query || filter ? undefined : 'Add one above, or connect an app and it will save what matters.'}
        />
      )}

      {shown.length > 0 && (
        <ul className="mem-list">
          {shown.map((m) => (
            <li key={m.id} className={`mem-row${editing === m.id ? ' editing' : ''}`}>
              <AppLogo app={m.via || 'command line'} size={32} />
              <div className="mem-body">
                {editing === m.id ? (
                  <textarea
                    ref={editRef}
                    aria-label="Edit memory"
                    value={edit}
                    rows={1}
                    onChange={(e) => setEdit(e.target.value)}
                    onBlur={() => save(m)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' && !e.shiftKey) {
                        e.preventDefault();
                        save(m);
                      } else if (e.key === 'Escape') {
                        setEditing('');
                      }
                    }}
                  />
                ) : (
                  <button
                    className="mem-text"
                    title="Click to edit"
                    onClick={() => {
                      setEditing(m.id);
                      setEdit(m.content);
                    }}
                  >
                    {m.content}
                  </button>
                )}
                <span className="mem-meta">{[m.project, m.via, ago(m.updated_at)].filter(Boolean).join(' · ')}</span>
              </div>
              <button className="mem-forget" aria-label={`Forget ${m.content}`} title="Forget" onClick={() => forget(m)}>
                <Icon d={I.trash} size={16} />
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
