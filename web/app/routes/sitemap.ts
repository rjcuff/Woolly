import { source } from '@/lib/source';
import { siteUrl } from '@/lib/shared';

export function loader() {
  const urls = ['/', ...source.getPages().map((p) => p.url), '/privacy', '/terms'];
  const body = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
${urls.map((u) => `  <url><loc>${new URL(u, siteUrl).toString()}</loc></url>`).join('\n')}
</urlset>
`;
  return new Response(body, { headers: { 'Content-Type': 'application/xml' } });
}
