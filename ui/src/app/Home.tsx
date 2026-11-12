import { type ReactNode, useCallback, useEffect, useMemo, useState } from 'react';
import { type Activity, type AppStatus, api, type Memory } from '../api';
import { AppLogo } from './logos';
import { Empty } from './Library';
import { ago, I, Skeleton, type Toast } from './ui';

const DAYS = 14;

function dayKey(d: Date) {
  return `${d.getFullYear()}-${d.getMonth()}-${d.getDate()}`;
}

/** The home screen: what your AI tools know and what they've been doing. */
export function Home({
  onOpenTab,
  onOpenConflicts,
  onToast,
  refresh,
}: {
  onOpenTab: (tab: 'apps' | 'memories') => void;
  onOpenConflicts: () => void;
  onToast: (t: Toast) => void;
  refresh: number;
}) {
  const [mems, setMems] = useState<Memory[] | null>(null);
  const [log, setLog] = useState<Activity[]>([]);
  const [apps, setApps] = useState<AppStatus[]>([]);

  const load = useCallback(async () => {
    try {
      const [all, activity, appList] = await Promise.all([api.list(true), api.activity(2000), api.apps().catch(() => [])]);
      setMems(all);
      setLog(activity);
      setApps(appList);
    } catch (e) {
      onToast({ text: (e as Error).message });
    }
  }, [onToast]);

  // biome-ignore lint/correctness/useExhaustiveDependencies: refresh is a signal to reload.
  useEffect(() => {
    load();
  }, [load, refresh]);

  const stats = useMemo(() => {
    const all = mems ?? [];
    const active = all.filter((m) => m.status === 'active');
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    const first = all.reduce((t, m) => Math.min(t, new Date(m.created_at).getTime()), Date.now());
    const age = Math.ceil((today.getTime() - first) / 86400000) + 1;
    const span = age < 7 ? 7 : DAYS;
    const days = Array.from({ length: span }, (_, i) => {
      const d = new Date(today);
      d.setDate(d.getDate() - (span - 1 - i));
      return { date: d, count: 0 };
    });
    const index = new Map(days.map((d, i) => [dayKey(d.date), i]));
    for (const m of all) {
      const i = index.get(dayKey(new Date(m.created_at)));
      if (i !== undefined) days[i].count++;
    }
    const weekAgo = Date.now() - 7 * 86400 * 1000;
    const byApp = new Map<string, number>();
    for (const e of log) {
      if (new Date(e.time).getTime() < weekAgo || e.app === 'review page') continue;
      byApp.set(e.app, (byApp.get(e.app) ?? 0) + 1);
    }
    const projects = new Map<string, number>();
    for (const m of active) if (m.project) projects.set(m.project, (projects.get(m.project) ?? 0) + 1);
    return {
      active,
      conflicts: all.filter((m) => m.status === 'conflict').length,
      days,
      week: days.slice(-7).reduce((n, d) => n + d.count, 0),
      busiest: [...byApp.entries()].sort((a, b) => b[1] - a[1]).slice(0, 5),
      projects: [...projects.entries()].sort((a, b) => b[1] - a[1]).slice(0, 5),
      recent: active.slice(0, 5),
    };
  }, [mems, log]);

  const forget = async (m: Memory) => {
    try {
      await api.forget(m.id);
      load();
      onToast({
        text: 'Forgotten',
        undo: async () => {
          await api.restore(m.id);
          load();
        },
      });
    } catch (e) {
      onToast({ text: (e as Error).message });
    }
  };

  if (!mems) return <HomeSkeleton />;
  const connected = apps.filter((a) => a.connected);
  const fmt = (d: Date) => d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
  const month = new Date().toLocaleDateString('en-US', { month: 'long', year: 'numeric' });

  return (
    <div className="ca-page">
      <div className="ca-title-row">
        <h1 className="ca-title">Home</h1>
        <span className="ca-title-aside">{month}</span>
      </div>

      <section className="sec">
        <h2 className="sec-title">
          Memories
          <button className="btn plain" onClick={() => onOpenTab('memories')}>
            See all
          </button>
        </h2>
        <p className="sec-sub">
          Saved per day, {fmt(stats.days[0].date)} to {fmt(stats.days[stats.days.length - 1].date)}
        </p>
        <div className="kpis">
          <Kpi label="Total" value={stats.active.length.toLocaleString()} />
          <Kpi label="This week" value={stats.week} />
          <Kpi label="Apps" value={connected.length} />
          <Kpi
            label="To review"
            value={
              stats.conflicts > 0 ? (
                <button className="kpi-link" onClick={onOpenConflicts}>
                  {stats.conflicts}
                </button>
              ) : (
                0
              )
            }
          />
        </div>
        <ColumnChart days={stats.days} />
      </section>

      <div className="two">
        <section>
          <h2 className="sec-title">Recent</h2>
          {stats.recent.length === 0 ? (
            <Empty icon={I.memory} title="No memories yet" text="Connect an app, and what it learns about you shows up here.">
              <button className="btn primary" onClick={() => onOpenTab('apps')}>
                Connect an app
              </button>
              <button className="btn secondary" onClick={() => onOpenTab('memories')}>
                Add one
              </button>
            </Empty>
          ) : (
            <ul className="list" style={{ marginTop: 12 }}>
              {stats.recent.map((m) => (
                <li key={m.id}>
                  <AppLogo app={m.via || 'command line'} size={28} />
                  <div className="list-text">
                    <p title={m.content}>{m.content}</p>
                    <span className="list-meta">{[m.via, m.project, ago(m.created_at)].filter(Boolean).join(' · ')}</span>
                  </div>
                  <button className="btn plain list-action" onClick={() => forget(m)}>
                    Forget
                  </button>
                </li>
              ))}
            </ul>
          )}
        </section>
        <section>
          <h2 className="sec-title">
            Most active
            <button className="btn plain" onClick={() => onOpenTab('apps')}>
              Apps
            </button>
          </h2>
          <p className="sec-sub">Last 7 days</p>
          {stats.busiest.length === 0 ? (
            <Empty icon={I.apps} title="Quiet week" text="Apps that use Woolly show up here." />
          ) : (
            <div style={{ marginTop: 16 }}>
              <BarList rows={stats.busiest} logos />
            </div>
          )}
          {stats.projects.length > 0 && (
            <>
              <h2 className="sec-title" style={{ marginTop: 40 }}>
                Projects
              </h2>
              <div style={{ marginTop: 16 }}>
                <BarList rows={stats.projects} />
              </div>
            </>
          )}
        </section>
      </div>
    </div>
  );
}

function Kpi({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div>
      <div className="kpi-label">{label}</div>
      <div className="kpi-value">{value}</div>
    </div>
  );
}

function HomeSkeleton() {
  return (
    <div className="ca-page home-skel" aria-busy="true">
      <div className="ca-title-row">
        <h1 className="ca-title">Home</h1>
      </div>
      <Skeleton w={120} h={22} />
      <div className="kpis">
        {[0, 1, 2, 3].map((i) => (
          <div key={i}>
            <Skeleton w={60} h={12} />
            <Skeleton w={44} h={24} r={8} />
          </div>
        ))}
      </div>
      <Skeleton h={220} r={12} />
      <div className="two" style={{ marginTop: 48 }}>
        <div>
          {[0, 1, 2, 3].map((i) => (
            <Skeleton key={i} h={36} r={10} className="skel-row" />
          ))}
        </div>
        <div>
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} h={28} r={8} className="skel-row" />
          ))}
        </div>
      </div>
    </div>
  );
}

/** Memories saved per day. One series, so no legend; the tile label names it. */
function ColumnChart({ days }: { days: { date: Date; count: number }[] }) {
  const [hover, setHover] = useState<number | null>(null);
  const empty = days.every((d) => d.count === 0);
  if (empty) {
    return (
      <div className="cols-empty">
        <div className="ghost" aria-hidden="true">
          {days.map((d, i) => (
            <span key={d.date.toISOString()} style={{ height: `${25 + ((i * 37) % 55)}%` }} />
          ))}
        </div>
        <p>Your chart fills in as apps save memories.</p>
      </div>
    );
  }
  const top = Math.max(2, ...days.map((d) => d.count));
  const max = top % 2 === 0 ? top : top + 1;
  const ticks = [0, max / 2, max];
  const fmt = (d: Date) => d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
  return (
    <figure className="cols" aria-label={`Memories saved per day for the last ${days.length} days`}>
      <div className="cols-y" aria-hidden="true">
        {ticks.map((t) => (
          <span key={t} style={{ bottom: `${(t / max) * 100}%` }}>
            {t}
          </span>
        ))}
      </div>
      <div className="cols-plot" onMouseLeave={() => setHover(null)}>
        {ticks.slice(1).map((t) => (
          <span key={t} className="cols-grid" style={{ bottom: `${(t / max) * 100}%` }} />
        ))}
        {days.map((d, i) => (
          <div
            key={d.date.toISOString()}
            className="col-slot"
            onMouseEnter={() => setHover(i)}
            onFocus={() => setHover(i)}
            onBlur={() => setHover(null)}
            tabIndex={0}
            aria-label={`${fmt(d.date)}: ${d.count} saved`}
          >
            {d.count > 0 && (
              <div
                className={`col${hover === i ? ' hot' : ''}`}
                style={{ height: `${(d.count / max) * 100}%`, animationDelay: `${i * 25}ms` }}
              />
            )}
            {hover === i && (
              <span className="col-tip" role="tooltip" style={{ bottom: `calc(${(d.count / max) * 100}% + 8px)` }}>
                <strong>{d.count}</strong> saved · {fmt(d.date)}
              </span>
            )}
          </div>
        ))}
      </div>
      <div className="cols-x" aria-hidden="true">
        {days.map((d, i) => (
          // On narrow screens only every other date fits.
          <span key={d.date.toISOString()} className={(days.length - 1 - i) % 2 ? 'x-odd' : undefined}>
            {fmt(d.date)}
          </span>
        ))}
      </div>
      <table className="sr-only">
        <tbody>
          {days.map((d) => (
            <tr key={d.date.toISOString()}>
              <th>{fmt(d.date)}</th>
              <td>{d.count}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </figure>
  );
}

/** Ranked rows with a thin bar under each. */
function BarList({ rows, logos = false }: { rows: [string, number][]; logos?: boolean }) {
  const max = Math.max(1, ...rows.map((r) => r[1]));
  return (
    <ul className="bars">
      {rows.map(([label, n]) => (
        <li key={label} style={logos ? undefined : { gridTemplateColumns: 'minmax(0, 1fr)' }}>
          {logos && <AppLogo app={label} size={24} />}
          <div className="bar-body">
            <div className="bar-top">
              <span className="bar-label">{label}</span>
              <span className="bar-value">{n}</span>
            </div>
            <div className="bar-track">
              <div className="bar" style={{ width: `${(n / max) * 100}%` }} />
            </div>
          </div>
        </li>
      ))}
    </ul>
  );
}
