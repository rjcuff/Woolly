import { appName, siteUrl } from './shared';

type SeoInput = {
  title: string;
  description: string;
  path: string;
  image?: string;
  type?: 'website' | 'article';
};

// Tags for search engines and link previews. Titles never use colons or dashes.
export function seo({ title, description, path, image, type = 'website' }: SeoInput) {
  const url = new URL(path, siteUrl).toString();
  const imageUrl = image ? new URL(image, siteUrl).toString() : undefined;
  return [
    { title },
    { name: 'description', content: description },
    { tagName: 'link', rel: 'canonical', href: url },
    { property: 'og:site_name', content: appName },
    { property: 'og:type', content: type },
    { property: 'og:title', content: title },
    { property: 'og:description', content: description },
    { property: 'og:url', content: url },
    ...(imageUrl ? [{ property: 'og:image', content: imageUrl }] : []),
    { name: 'twitter:card', content: imageUrl ? 'summary_large_image' : 'summary' },
    { name: 'twitter:title', content: title },
    { name: 'twitter:description', content: description },
    ...(imageUrl ? [{ name: 'twitter:image', content: imageUrl }] : []),
  ];
}
