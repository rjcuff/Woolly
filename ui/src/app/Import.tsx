import { useRef, useState } from 'react';
import { api, type ImportItem, type ImportPreview, importPreview } from '../api';
import { AppLogo } from './logos';
import { I, Icon, Skeleton, type Toast } from './ui';

type Stage = { name: 'pick' } | { name: 'reading'; progress: number; file: string } | { name: 'review'; preview: ImportPreview } | { name: 'done'; added: number; skipped: number };

/** Brings facts about you out of a ChatGPT or Claude data export. */
export function Import({ onBack, onToast, onChanged }: { onBack: () => void; onToast: (t: Toast) => void; onChanged: () => void }) {
  const [stage, setStage] = useState<Stage>({ name: 'pick' });
  const [picked, setPicked] = useState<Set<number>>(new Set());
  const [dragging, setDragging] = useState(false);
  const [saving, setSaving] = useState(false);
  const input = useRef<HTMLInputElement>(null);

  const read = async (file: File) => {
    setStage({ name: 'reading', progress: 0, file: file.name });
    try {
      const preview = await importPreview(file, (p) => setStage({ name: 'reading', progress: p, file: file.name }));
      setPicked(new Set(preview.items.map((_, i) => i).filter((i) => !preview.items[i].known)));
      setStage({ name: 'review', preview });
    } catch (e) {
      onToast({ text: (e as Error).message });
      setStage({ name: 'pick' });
    }
  };

  const save = async (preview: ImportPreview) => {
    const items: ImportItem[] = preview.items.filter((_, i) => picked.has(i));
    setSaving(true);
    try {
      const r = await api.importSave(preview.provider, items);
      onChanged();
      setStage({ name: 'done', added: r.added, skipped: r.known + r.blocked });
    } catch (e) {
      onToast({ text: (e as Error).message });
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="ca-page narrow">
      <button className="back" onClick={onBack}>
        <Icon d={I.chevronLeft} size={18} />
        Memories
      </button>

      {stage.name === 'pick' && (
        <>
          <h1 className="ca-title big">Bring your memory with you</h1>
          <p className="import-lede">Woolly reads what you told ChatGPT or Claude and keeps the facts about you. You choose what to save, and nothing leaves this computer.</p>
          <button
            type="button"
            className={`drop${dragging ? ' over' : ''}`}
            onClick={() => input.current?.click()}
            onDragOver={(e) => {
              e.preventDefault();
              setDragging(true);
            }}
            onDragLeave={() => setDragging(false)}
            onDrop={(e) => {
              e.preventDefault();
              setDragging(false);
              const f = e.dataTransfer.files[0];
              if (f) read(f);
            }}
          >
            <span className="drop-logos">
              <AppLogo app="chatgpt" size={44} />
              <AppLogo app="claude" size={44} />
            </span>
            <strong>Drop your export here</strong>
            <span>or click to choose the .zip file</span>
          </button>
          <input
            ref={input}
            type="file"
            accept=".zip,.json,application/zip,application/json"
            hidden
            onChange={(e) => {
              const f = e.target.files?.[0];
              if (f) read(f);
              e.target.value = '';
            }}
          />
          <div className="how">
            <div>
              <strong>From ChatGPT</strong>
              <span>Settings → Data controls → Export data. You’ll get an email with a .zip.</span>
            </div>
            <div>
              <strong>From Claude</strong>
              <span>Settings → Privacy → Export data. You’ll get an email with a .zip.</span>
            </div>
          </div>
        </>
      )}

      {stage.name === 'reading' && (
        <>
          <h1 className="ca-title big">Reading your history</h1>
          <p className="import-lede">{stage.progress < 1 ? `Uploading ${stage.file}…` : 'Finding the facts about you…'}</p>
          <div className="bar-track import-progress">
            <div className="bar" style={{ width: `${Math.max(4, stage.progress * 100)}%` }} />
          </div>
          <ul className="mem-list">
            {[0, 1, 2, 3].map((i) => (
              <li key={i} className="mem-row">
                <Skeleton w={20} h={20} r={6} />
                <div className="mem-body">
                  <Skeleton w={`${75 - i * 10}%`} h={14} />
                </div>
              </li>
            ))}
          </ul>
        </>
      )}

      {stage.name === 'review' && stage.preview.items.length === 0 && (
        <>
          <h1 className="ca-title big">Nothing to bring over</h1>
          <p className="import-lede">
            Woolly read {stage.preview.messages.toLocaleString()} of your messages but didn’t find facts about you, like where you live or what you prefer.
          </p>
          <button className="btn secondary" onClick={() => setStage({ name: 'pick' })}>
            Try another file
          </button>
        </>
      )}

      {stage.name === 'review' && stage.preview.items.length > 0 && (
        <>
          <div className="ca-title-row">
            <h1 className="ca-title big">
              Found {stage.preview.items.length} {stage.preview.items.length === 1 ? 'fact' : 'facts'}
            </h1>
            <AppLogo app={stage.preview.provider} size={32} />
          </div>
          <p className="import-lede">From {stage.preview.messages.toLocaleString()} of your messages. Uncheck anything you don’t want Woolly to keep.</p>
          <div className="import-tools">
            <button className="btn plain" onClick={() => setPicked(new Set(stage.preview.items.map((_, i) => i)))}>
              Select all
            </button>
            <button className="btn plain" onClick={() => setPicked(new Set())}>
              Select none
            </button>
          </div>
          <ul className="mem-list">
            {stage.preview.items.map((it, i) => (
              <li key={`${i}-${it.content}`} className="mem-row">
                <label className="pick">
                  <input
                    type="checkbox"
                    checked={picked.has(i)}
                    onChange={(e) => {
                      const next = new Set(picked);
                      if (e.target.checked) next.add(i);
                      else next.delete(i);
                      setPicked(next);
                    }}
                  />
                  <span className="mem-body">
                    <span>{it.content}</span>
                    {it.known && <span className="mem-meta">Already in Woolly</span>}
                  </span>
                </label>
              </li>
            ))}
          </ul>
          <div className="import-bar">
            <button className="btn secondary" onClick={() => setStage({ name: 'pick' })}>
              Cancel
            </button>
            <button className="btn primary" disabled={picked.size === 0 || saving} onClick={() => save(stage.preview)}>
              {saving ? 'Saving…' : `Save ${picked.size} ${picked.size === 1 ? 'memory' : 'memories'}`}
            </button>
          </div>
        </>
      )}

      {stage.name === 'done' && (
        <div className="empty">
          <span className="empty-icon done">
            <Icon d={I.check} size={24} />
          </span>
          <p className="empty-title">
            Saved {stage.added} {stage.added === 1 ? 'memory' : 'memories'}
          </p>
          <p className="empty-text">
            Every connected app can use them now.{stage.skipped > 0 ? ` ${stage.skipped} were already known or looked like passwords, so they were skipped.` : ''}
          </p>
          <div className="empty-actions">
            <button className="btn primary" onClick={onBack}>
              See them
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
