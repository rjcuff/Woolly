import { createFromSource } from 'fumadocs-core/search/server';
import { source } from '@/lib/source';

const server = createFromSource(source, {
  language: 'english',
});

// Built once into a static index that the browser searches locally.
export async function loader() {
  return server.staticGET();
}
