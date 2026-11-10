import type { Config } from '@react-router/dev/config';
import { glob } from 'node:fs/promises';
import { createGetUrl, getSlugs } from 'fumadocs-core/source';
import { getPageImageUrl, getPageMarkdownUrl } from './app/lib/shared';

const getUrl = createGetUrl('/docs');

// The whole site is static: every page, preview image, the search index and
// the markdown copies are rendered at build time into build/client.
export default {
  ssr: false,
  async prerender({ getStaticPaths }) {
    const paths = new Set<string>(getStaticPaths());
    paths.add('/api/search');
    paths.add('/sitemap.xml');
    paths.add('/og/docs/profile.png');

    for await (const entry of glob('**/*.mdx', { cwd: '../docs' })) {
      const slugs = getSlugs(entry);
      paths.add(getUrl(slugs));
      paths.add(getPageImageUrl({ slugs }).url);
      paths.add(getPageMarkdownUrl({ slugs }).url);
    }

    return [...paths];
  },
} satisfies Config;
