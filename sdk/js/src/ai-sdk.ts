// Memory for the Vercel AI SDK. Wrap any model and every call gets the
// user's relevant memories as a system message:
//
//   import { wrapLanguageModel } from 'ai';
//   import { woollyMemory } from 'woolly-ai/ai-sdk';
//   const model = wrapLanguageModel({ model: openai('gpt-5'), middleware: woollyMemory() });

import { Woolly, type WoollyOptions } from './index.js';

type Part = { type: string; text?: string };
type Message = { role: string; content: string | Part[] };

export interface WoollyMemoryOptions extends WoollyOptions {
  /** Memories to add per call. Default 6. */
  limit?: number;
  /** Search this project's memories plus the user's. */
  project?: string;
  /** Use your own client instead of creating one. */
  client?: Woolly;
}

function lastUserText(prompt: Message[]): string {
  for (let i = prompt.length - 1; i >= 0; i--) {
    const m = prompt[i];
    if (m.role !== 'user') continue;
    if (typeof m.content === 'string') return m.content;
    return m.content
      .filter((p) => p.type === 'text' && p.text)
      .map((p) => p.text)
      .join('\n');
  }
  return '';
}

/**
 * AI SDK middleware that adds the user's relevant Woolly memories to each
 * call. If Woolly isn't running, calls go through unchanged.
 */
export function woollyMemory(options: WoollyMemoryOptions = {}) {
  const woolly = options.client ?? new Woolly(options);
  return {
    specificationVersion: 'v4' as const,
    async transformParams<P extends { prompt: Message[] }>({ params }: { params: P }): Promise<P> {
      const query = lastUserText(params.prompt).trim();
      if (!query) return params;
      let note = '';
      try {
        note = await woolly.context(query, { limit: options.limit ?? 6, project: options.project });
      } catch {
        return params;
      }
      if (!note) return params;
      return { ...params, prompt: [{ role: 'system', content: note }, ...params.prompt] };
    },
  };
}
