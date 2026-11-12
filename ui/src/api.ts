export type Source = 'user' | 'import' | 'agent';
export type Status = 'active' | 'superseded' | 'conflict';

export type Memory = {
  id: string;
  content: string;
  key?: string;
  tags?: string[];
  source: Source;
  status: Status;
  created_at: string;
  updated_at: string;
  superseded_by?: string;
  conflicts_with?: string;
  via?: string; // the app that saved it
  project?: string; // set for facts that only matter in one project
  inTrash?: boolean; // set by the page for memories shown in the trash
};

export type Activity = {
  time: string;
  app: string;
  action: string;
  ids?: string[];
  query?: string;
  count?: number;
};

export type AppStatus = {
  id: string;
  name: string;
  installed: boolean;
  connected: boolean;
  restart?: boolean;
  last_used?: string;
};
export type Peer = { addr: string; name?: string; last_sync?: string; last_error?: string };
export type Settings = {
  version: string;
  home: string;
  ollama?: {
    host: string;
    running: boolean;
    version?: string;
    chat_model: string;
    embed_model: string;
    has_chat_model?: boolean;
    has_embed_model?: boolean;
  };
  sync: { enabled: boolean; name?: string; peers?: Peer[]; addresses?: string[]; port?: string; error?: string };
};
export type SyncResult = { addr: string; name?: string; received: number; sent: number; error?: string };

export type RecallResult = { memory: Memory; score: number };

async function call<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...init,
    // Names this page in the activity log, so its own reads aren't logged.
    headers: { ...(init?.body ? { 'Content-Type': 'application/json' } : {}), 'X-Woolly-Client': 'review page' },
  });
  const text = await res.text();
  const data = text ? JSON.parse(text) : {};
  if (res.status === 401) throw new Error('This page needs to be opened with `woolly ui`. Run it in your terminal.');
  if (!res.ok) throw new Error(data.error ?? `Request failed (${res.status})`);
  return data as T;
}

export const api = {
  list: (all: boolean) => call<{ memories: Memory[] }>(`/api/memories${all ? '?all=1' : ''}`).then((r) => r.memories),
  conflicts: () => call<{ conflicts: Memory[] }>('/api/conflicts').then((r) => r.conflicts),
  recall: (query: string, includeAll: boolean) =>
    call<{ results: RecallResult[] }>('/api/recall', {
      method: 'POST',
      body: JSON.stringify({ query, limit: 50, include_all: includeAll }),
    }).then((r) => r.results.map((x) => x.memory)),
  remember: (content: string, key: string, tags: string[]) =>
    call<{ memory: Memory }>('/api/remember', {
      method: 'POST',
      body: JSON.stringify({ content, key, tags, source: 'user' }),
    }),
  update: (id: string, patch: { content?: string; key?: string; tags?: string[]; project?: string }) =>
    call<Memory>(`/api/memories/${encodeURIComponent(id)}`, { method: 'PATCH', body: JSON.stringify(patch) }),
  forget: (id: string) => call<unknown>(`/api/memories/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  trash: () => call<{ trash: Memory[] }>('/api/trash').then((r) => r.trash),
  restore: (id: string) => call<Memory>('/api/restore', { method: 'POST', body: JSON.stringify({ id }) }),
  emptyTrash: () => call<{ deleted: number }>('/api/trash', { method: 'DELETE' }),
  resolve: (keepId: string) =>
    call<unknown>('/api/resolve', { method: 'POST', body: JSON.stringify({ keep_id: keepId }) }),
  activity: (limit = 300) => call<{ activity: Activity[] }>(`/api/activity?limit=${limit}`).then((r) => r.activity),
  apps: () => call<{ apps: AppStatus[] }>('/api/apps').then((r) => r.apps),
  setApp: (id: string, connect: boolean) =>
    call<{ result: string }>(`/api/apps/${encodeURIComponent(id)}/${connect ? 'connect' : 'disconnect'}`, { method: 'POST' }),
  settings: () => call<Settings>('/api/settings'),
  syncInit: () => call<{ code: string; addresses: string[] | null }>('/api/sync/init', { method: 'POST', body: '{}' }),
  syncJoin: (addr: string, code: string) =>
    call<{ results: SyncResult[] | null }>('/api/sync/join', { method: 'POST', body: JSON.stringify({ addr, code }) }),
  syncNow: () => call<{ results: SyncResult[] | null }>('/api/sync/now', { method: 'POST', body: '{}' }),
  syncLeave: () => call<unknown>('/api/sync/leave', { method: 'POST', body: '{}' }),
  profile: async () => {
    const res = await fetch('/api/profile?format=markdown', { headers: { 'X-Woolly-Client': 'review page' } });
    if (res.status === 401) throw new Error('This page needs to be opened with `woolly ui`. Run it in your terminal.');
    return res.text();
  },
};

export function splitTags(s: string): string[] {
  return s
    .split(',')
    .map((t) => t.trim())
    .filter(Boolean);
}
