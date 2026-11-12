import { type ReactNode, useCallback, useEffect, useState } from 'react';
import { api, type Settings as SettingsData } from '../api';
import type { LibraryView } from './Library';
import { ago, I, Icon, Skeleton, type Toast } from './ui';

/** Ollama, sync between devices, and where your memories live. */
export function Settings({ onToast, onOpen }: { onToast: (t: Toast) => void; onOpen: (view: LibraryView) => void }) {
  const [s, setS] = useState<SettingsData | null>(null);
  const [code, setCode] = useState('');
  const [addrs, setAddrs] = useState<string[]>([]);
  const [join, setJoin] = useState({ addr: '', code: '' });
  const [joining, setJoining] = useState(false);
  const [busy, setBusy] = useState(false);

  const load = useCallback(() => {
    api
      .settings()
      .then(setS)
      .catch((e: Error) => onToast({ text: e.message }));
  }, [onToast]);

  useEffect(load, [load]);

  const run = async (f: () => Promise<unknown>, done?: string) => {
    setBusy(true);
    try {
      await f();
      if (done) onToast({ text: done });
      load();
      return true;
    } catch (e) {
      onToast({ text: (e as Error).message });
      return false;
    } finally {
      setBusy(false);
    }
  };

  const showCode = () =>
    run(async () => {
      const r = await api.syncInit();
      setCode(r.code);
      setAddrs(r.addresses ?? []);
    });

  if (!s) {
    return (
      <div className="ca-page narrow" aria-busy="true">
        <h1 className="ca-title">Settings</h1>
        {[0, 1, 2].map((g) => (
          <div key={g} style={{ marginBottom: 44 }}>
            <Skeleton w={100} h={20} className="skel-row" />
            {[0, 1, 2].map((i) => (
              <Skeleton key={i} h={36} r={8} className="skel-row" />
            ))}
          </div>
        ))}
      </div>
    );
  }

  const o = s.ollama;
  const peers = s.sync.peers ?? [];
  const joinCmd = `woolly sync join ${addrs[0] ?? '<this-computer>'} ${code}`;

  return (
    <div className="ca-page narrow">
      <h1 className="ca-title">Settings</h1>

      <h2 className="group-label">Ollama</h2>
      <div className="set-group">
        <Row label="Status" value={o?.running ? <Ok>Running</Ok> : <Warn>Not running</Warn>} />
        {o?.running && (
          <>
            <Row label="Learning" value={o.has_chat_model ? <Ok>{o.chat_model}</Ok> : <Pull model={o.chat_model} />} />
            <Row label="Search" value={o.has_embed_model ? <Ok>{o.embed_model}</Ok> : <Pull model={o.embed_model} />} />
          </>
        )}
        {!o?.running && <Row label="Get Ollama" value={<a href="https://ollama.com" target="_blank" rel="noreferrer">ollama.com</a>} />}
      </div>

      <h2 className="group-label">Sync</h2>
      <div className="set-group">
        {s.sync.enabled ? (
          <>
            <Row label="This device" value={s.sync.name ?? ''} />
            {peers.map((p) => (
              <Row
                key={p.addr}
                label={p.name || p.addr}
                value={p.last_error ? <Warn>Can’t reach it</Warn> : p.last_sync ? `Synced ${ago(p.last_sync)}` : 'Waiting'}
              />
            ))}
            <div className="set-actions">
              {peers.length > 0 && (
                <button className="btn primary" disabled={busy} onClick={() => run(api.syncNow, 'Synced')}>
                  Sync now
                </button>
              )}
              <button className="btn secondary" disabled={busy} onClick={showCode}>
                Add a device
              </button>
              <button className="btn plain" disabled={busy} onClick={() => run(api.syncLeave, 'Sync off')}>
                Stop syncing
              </button>
            </div>
          </>
        ) : (
          <div className="set-actions">
            <button className="btn primary" disabled={busy} onClick={showCode}>
              Set up sync
            </button>
            <button className="btn secondary" disabled={busy} onClick={() => setJoining(true)}>
              Join a device
            </button>
          </div>
        )}
        {code && (
          <div className="set-code">
            <code>{joinCmd}</code>
            <button
              className="btn plain"
              onClick={() =>
                navigator.clipboard
                  .writeText(joinCmd)
                  .then(() => onToast({ text: 'Copied' }))
                  .catch(() => onToast({ text: 'Select it to copy' }))
              }
            >
              Copy
            </button>
          </div>
        )}
        {(joining || s.sync.enabled) && (
          <form
            className="set-join"
            onSubmit={async (e) => {
              e.preventDefault();
              if (await run(() => api.syncJoin(join.addr, join.code), 'Linked')) {
                setJoin({ addr: '', code: '' });
                setJoining(false);
              }
            }}
          >
            <input aria-label="Address" placeholder="Address" value={join.addr} onChange={(e) => setJoin({ ...join, addr: e.target.value })} />
            <input aria-label="Code" placeholder="Code" value={join.code} onChange={(e) => setJoin({ ...join, code: e.target.value })} />
            <button className="btn secondary" disabled={busy || !join.addr || !join.code}>
              Join
            </button>
          </form>
        )}
      </div>

      <h2 className="group-label">Memories</h2>
      <div className="link-cards">
        {(
          [
            ['conflicts', 'Conflicts', I.alert],
            ['history', 'History', I.clock],
            ['trash', 'Trash', I.trash],
            ['activity', 'Activity', I.ask],
          ] as [LibraryView, string, string][]
        ).map(([view, label, icon]) => (
          <button key={view} className="link-card" onClick={() => onOpen(view)}>
            <span className="link-icon">
              <Icon d={icon} size={17} />
            </span>
            {label}
            <Icon d={I.chevronRight} size={16} />
          </button>
        ))}
      </div>
      <div className="set-group" style={{ marginTop: 12 }}>
        <Row label="Stored in" value={<code>{s.home}</code>} />
      </div>

      <p className="set-foot">Woolly {s.version}</p>
    </div>
  );
}

function Row({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="set-row">
      <span className="set-label">{label}</span>
      <span className="set-value">{value}</span>
    </div>
  );
}

function Ok({ children }: { children: ReactNode }) {
  return (
    <span className="ok">
      <Icon d={I.check} size={14} /> {children}
    </span>
  );
}

function Warn({ children }: { children: ReactNode }) {
  return (
    <span className="warn">
      <Icon d={I.alert} size={14} /> {children}
    </span>
  );
}

function Pull({ model }: { model: string }) {
  return (
    <span className="warn">
      <Icon d={I.alert} size={14} /> <code>ollama pull {model}</code>
    </span>
  );
}
