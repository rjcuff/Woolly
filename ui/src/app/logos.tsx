// App logos from Lobe Icons and Devicon (MIT) and Simple Icons (CC0), in
// ui/public/logos. Each logo is a trademark of its owner, shown to say Woolly
// works with that app.

const FILES = new Set(['claude', 'claudecode', 'codex', 'cursor', 'windsurf', 'geminicli', 'lmstudio', 'ollama', 'opencode', 'vscode', 'python']);

// App IDs and the names apps use in the activity log, mapped to a logo.
const ALIASES: Record<string, string> = {
  'claude-code': 'claudecode',
  'claude code': 'claudecode',
  'claude-desktop': 'claude',
  'claude desktop': 'claude',
  gemini: 'geminicli',
  'gemini cli': 'geminicli',
  'lm studio': 'lmstudio',
  'vs code': 'vscode',
  'python app': 'python',
  proxy: 'ollama',
};

/** A rounded app tile with the app's logo, or its initials. */
export function AppLogo({ app, size = 32 }: { app: string; size?: number }) {
  const k = app.toLowerCase();
  const key = ALIASES[k] ?? k;
  const radius = size * 0.26;
  if (FILES.has(key)) {
    return (
      <span className="app-logo file" style={{ width: size, height: size, borderRadius: radius }} aria-hidden="true">
        <img src={`/logos/${key}.svg`} alt="" width={size * 0.62} height={size * 0.62} />
      </span>
    );
  }
  if (key === 'woolly chat' || key === 'review page' || key === 'woolly') {
    return (
      <span className="app-logo file" style={{ width: size, height: size, borderRadius: radius }} aria-hidden="true">
        <img src="/mammoth.png" alt="" width={size * 0.72} height={size * 0.72} />
      </span>
    );
  }
  const text = key === 'command line' ? '>_' : app.replace(/[^A-Za-z0-9]/g, '').slice(0, 2).toUpperCase() || '?';
  return (
    <span className="app-logo mono" style={{ width: size, height: size, borderRadius: radius, fontSize: size * 0.36 }} aria-hidden="true">
      {text}
    </span>
  );
}
