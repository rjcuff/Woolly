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

export type ChatModel = { name: string; size: number; family?: string };
export type Saved = { id: string; fact: string; action: string; replaces?: string };
export type ChatMessage = { role: 'user' | 'assistant'; content: string; memories?: string[]; saved?: Saved[] };
export type Chat = { id: string; title: string; model?: string; messages: ChatMessage[] };
export type ChatSummary = { id: string; title: string; updated_at: string };
export type ChatEvent =
  | { type: 'start'; chat_id: string; memories: { id: string; content: string }[] | null }
  | { type: 'delta'; content: string }
  | { type: 'saved'; saved: Saved[] | null }
  | { type: 'done'; chat_id: string }
  | { type: 'error'; error: string };

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
  models: () => call<{ models: ChatModel[]; default: string }>('/api/models'),
  chats: () => call<{ chats: ChatSummary[] }>('/api/chats').then((r) => r.chats),
  chat: (id: string) => call<Chat>(`/api/chats/${encodeURIComponent(id)}`),
  deleteChat: (id: string) => call<unknown>(`/api/chats/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  send: async (
    body: { chat_id?: string; model: string; message: string; learn: boolean },
    onEvent: (e: ChatEvent) => void,
    signal?: AbortSignal,
  ) => {
    const res = await fetch('/api/chat', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'X-Woolly-Client': 'review page' },
      body: JSON.stringify(body),
      signal,
    });
    if (!res.ok || !res.body) {
      const data = await res.json().catch(() => ({}));
      throw new Error(data.error ?? `Request failed (${res.status})`);
    }
    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buf = '';
    for (;;) {
      const { value, done } = await reader.read();
      if (done) break;
      buf += decoder.decode(value, { stream: true });
      let nl = buf.indexOf('\n');
      while (nl >= 0) {
        const line = buf.slice(0, nl).trim();
        buf = buf.slice(nl + 1);
        if (line) onEvent(JSON.parse(line) as ChatEvent);
        nl = buf.indexOf('\n');
      }
    }
  },
  activity: () => call<{ activity: Activity[] }>('/api/activity?limit=300').then((r) => r.activity),
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
