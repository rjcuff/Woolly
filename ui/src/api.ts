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
  inTrash?: boolean; // set by the page for memories shown in the trash
};

export type RecallResult = { memory: Memory; score: number };

async function call<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: init?.body ? { 'Content-Type': 'application/json' } : undefined,
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
  update: (id: string, patch: { content?: string; key?: string; tags?: string[] }) =>
    call<Memory>(`/api/memories/${encodeURIComponent(id)}`, { method: 'PATCH', body: JSON.stringify(patch) }),
  forget: (id: string) => call<unknown>(`/api/memories/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  trash: () => call<{ trash: Memory[] }>('/api/trash').then((r) => r.trash),
  restore: (id: string) => call<Memory>('/api/restore', { method: 'POST', body: JSON.stringify({ id }) }),
  emptyTrash: () => call<{ deleted: number }>('/api/trash', { method: 'DELETE' }),
  resolve: (keepId: string) =>
    call<unknown>('/api/resolve', { method: 'POST', body: JSON.stringify({ keep_id: keepId }) }),
  profile: async () => {
    const res = await fetch('/api/profile?format=markdown');
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
