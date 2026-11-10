import type { Route } from './+types/og.docs';
import { source } from '@/lib/source';
import { tagline } from '@/lib/shared';
import { ogImage } from '@/lib/og';

export function loader({ params }: Route.LoaderArgs) {
  const slugs = params['*']
    .split('/')
    .filter((v) => v.length > 0)
    .slice(0, -1);
  const page = source.getPage(slugs);
  if (!page) throw new Response(undefined, { status: 404 });

  return ogImage(slugs.length === 0 ? tagline : page.data.title);
}
