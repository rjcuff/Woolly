import { createGetUrl } from 'fumadocs-core/source';

export const appName = 'Woolly';
export const tagline = 'Local memory for your AI tools';
export const siteDescription =
  'Woolly gives Claude, Cursor and your own tools one shared memory that lives in plain files on your laptop. Open source, private, and free.';
// The public URL of the site. Update this once the domain is live.
export const siteUrl = 'https://woolly.sh';
export const docsRoute = '/docs';
export const docsImageRoute = '/og/docs';
export const docsContentRoute = '/llms.mdx/docs';

export const gitConfig = {
  user: 'rjcuff',
  repo: 'wooly',
  branch: 'main',
};
export const githubUrl = `https://github.com/${gitConfig.user}/${gitConfig.repo}`;

const getContentUrl = createGetUrl(docsContentRoute);

export function getPageMarkdownUrl(page: { slugs: string[]; locale?: string }) {
  const segments = [...page.slugs, 'content.md'];

  return { segments, url: getContentUrl(segments, page.locale) };
}

const getImageUrl = createGetUrl(docsImageRoute);

export function getPageImageUrl(page: { slugs: string[]; locale?: string }) {
  const segments = [...page.slugs, 'image.webp'];

  return { segments, url: getImageUrl(segments, page.locale) };
}
