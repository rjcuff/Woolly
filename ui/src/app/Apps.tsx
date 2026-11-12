import { useCallback, useEffect, useState } from 'react';
import { type AppStatus, api } from '../api';
import { AppLogo } from './logos';
import { ago, Skeleton, type Toast } from './ui';

/** Every app Woolly can connect to, with one-click connect. */
export function Apps({ onToast }: { onToast: (t: Toast) => void }) {
  const [apps, setApps] = useState<AppStatus[] | null>(null);
  const [busy, setBusy] = useState('');

  const load = useCallback(() => {
    api
      .apps()
      .then(setApps)
      .catch((e: Error) => onToast({ text: e.message }));
  }, [onToast]);

  useEffect(load, [load]);

  const toggle = async (a: AppStatus) => {
    setBusy(a.id);
    try {
      await api.setApp(a.id, !a.connected);
      onToast({ text: a.connected ? `Disconnected ${a.name}` : a.restart ? `Connected. Restart ${a.name}.` : `Connected ${a.name}` });
      load();
    } catch (e) {
      onToast({ text: (e as Error).message });
    } finally {
      setBusy('');
    }
  };

  const here = apps?.filter((a) => a.installed) ?? [];
  const missing = apps?.filter((a) => !a.installed) ?? [];

  return (
    <div className="ca-page narrow">
      <h1 className="ca-title">Apps</h1>
      <h2 className="group-label">On this computer</h2>
      <ul className="group list-reset">
        {apps === null &&
          [0, 1, 2].map((i) => (
            <li key={i} className="app-row">
              <Skeleton w={36} h={36} r={9} />
              <div className="app-text">
                <Skeleton w={120} h={14} />
                <Skeleton w={80} h={12} className="skel-gap" />
              </div>
            </li>
          ))}
        {apps !== null && here.length === 0 && <li className="app-row app-state">None found</li>}
        {here.map((a) => (
          <li key={a.id} className="app-row">
            <AppLogo app={a.id} size={36} />
            <div className="app-text">
              <span className="app-name">{a.name}</span>
              <span className={`app-state${a.connected ? ' on' : ''}`}>
                {a.connected ? (a.last_used ? `Connected · ${ago(a.last_used)}` : 'Connected') : 'Not connected'}
              </span>
            </div>
            <button className={`btn ${a.connected ? 'secondary' : 'primary'}`} disabled={busy === a.id} onClick={() => toggle(a)}>
              {busy === a.id ? '…' : a.connected ? 'Disconnect' : 'Connect'}
            </button>
          </li>
        ))}
      </ul>
      {missing.length > 0 && (
        <>
          <h2 className="group-label">Not installed</h2>
          <ul className="group dim list-reset">
            {missing.map((a) => (
              <li key={a.id} className="app-row">
                <AppLogo app={a.id} size={36} />
                <div className="app-text">
                  <span className="app-name">{a.name}</span>
                </div>
              </li>
            ))}
          </ul>
        </>
      )}
    </div>
  );
}
