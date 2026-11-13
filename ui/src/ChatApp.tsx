import { useCallback, useEffect, useState } from 'react';
import { Apps } from './app/Apps';
import { Home } from './app/Home';
import { Import } from './app/Import';
import { Library, type LibraryView } from './app/Library';
import { Memories } from './app/Memories';
import { Settings } from './app/Settings';
import { I, Icon, IconButton, narrow, read, type Toast, write } from './app/ui';
import './chat-app.css';

type Tab = 'home' | 'memories' | 'apps' | 'settings';

const TABS: { id: Tab; label: string; icon: string }[] = [
  { id: 'home', label: 'Home', icon: I.home },
  { id: 'memories', label: 'Memories', icon: I.memory },
  { id: 'apps', label: 'Apps', icon: I.apps },
  { id: 'settings', label: 'Settings', icon: I.settings },
];

const SIDEBAR_KEY = 'woolly.sidebar';

function startTab(): Tab {
  const t = new URLSearchParams(window.location.search).get('tab');
  return TABS.some((x) => x.id === t) ? (t as Tab) : 'home';
}

/** The Woolly app: Home, Memories, Apps and Settings. */
export function ChatApp() {
  const [tab, setTab] = useState<Tab>(startTab);
  const [sidebar, setSidebar] = useState(() => !narrow() && read(SIDEBAR_KEY, '1') === '1');
  const [toast, setToast] = useState<Toast>(null);
  const [refresh, setRefresh] = useState(0);
  const [library, setLibrary] = useState<LibraryView | null>(null);
  const [importing, setImporting] = useState(false);
  const [version, setVersion] = useState('');

  const flash = useCallback((t: Toast) => {
    setToast(t);
    if (t) window.setTimeout(() => setToast((cur) => (cur === t ? null : cur)), t.undo ? 6000 : 3500);
  }, []);

  useEffect(() => {
    fetch('/api/version')
      .then((r) => r.json())
      .then((v) => setVersion(String(v.version).split(/[-+]/)[0]))
      .catch(() => {});
  }, []);

  const go = (t: Tab) => {
    setTab(t);
    setLibrary(null);
    setImporting(false);
    const url = new URL(window.location.href);
    url.searchParams.set('tab', t);
    window.history.replaceState(null, '', url);
    if (narrow()) setSidebar(false);
  };

  const toggleSidebar = () =>
    setSidebar((s) => {
      if (!narrow()) write(SIDEBAR_KEY, s ? '0' : '1');
      return !s;
    });

  const changed = useCallback(() => setRefresh((n) => n + 1), []);

  return (
    <div className={`ca${sidebar ? ' with-sidebar' : ''}`}>
      <aside className="ca-sidebar" aria-label="Navigation">
        <div className="ca-sidebar-top">
          <img className="ca-logo ca-mark" src="/mammoth.png" alt="Woolly" />
          <IconButton label="Close sidebar" onClick={toggleSidebar}>
            <Icon d={I.panel} />
          </IconButton>
        </div>
        <nav className="ca-nav" aria-label="Sections">
          {TABS.map((t) => (
            <button key={t.id} className={`ca-row${tab === t.id ? ' active' : ''}`} aria-current={tab === t.id ? 'page' : undefined} onClick={() => go(t.id)}>
              <Icon d={t.icon} />
              {t.label}
            </button>
          ))}
        </nav>
        <div className="ca-sidebar-foot">
          <div className="ca-me">
            <img className="ca-mark" src="/mammoth.png" alt="" />
            <div>
              <strong>Woolly</strong>
              <span>{version ? `Local · ${version}` : 'Local'}</span>
            </div>
          </div>
        </div>
      </aside>
      {sidebar && <div className="ca-scrim" onClick={toggleSidebar} aria-hidden="true" />}

      <main className="ca-main">
        {!sidebar && (
          <header className="ca-header">
            <IconButton label="Open sidebar" onClick={toggleSidebar}>
              <Icon d={I.panel} />
            </IconButton>
          </header>
        )}
        {tab === 'home' && (
          <Home
            onOpenTab={go}
            onOpenConflicts={() => {
              go('settings');
              setLibrary('conflicts');
            }}
            onImport={() => {
              go('memories');
              setImporting(true);
            }}
            onToast={flash}
            refresh={refresh}
          />
        )}
        {tab === 'memories' && !importing && <Memories onToast={flash} onChanged={changed} refresh={refresh} onImport={() => setImporting(true)} />}
        {tab === 'memories' && importing && <Import onBack={() => setImporting(false)} onToast={flash} onChanged={changed} />}
        {tab === 'apps' && <Apps onToast={flash} />}
        {tab === 'settings' && !library && <Settings onToast={flash} onOpen={setLibrary} />}
        {tab === 'settings' && library && <Library view={library} onBack={() => setLibrary(null)} onToast={flash} onChanged={changed} />}
      </main>

      <div className={`ca-toast${toast ? ' shown' : ''}`} role="status">
        {toast && (
          <>
            <span>{toast.text}</span>
            {toast.undo && (
              <button
                className="link-btn"
                onClick={() => {
                  toast.undo?.();
                  setToast(null);
                }}
              >
                Undo
              </button>
            )}
          </>
        )}
      </div>
    </div>
  );
}
