import type { Route } from './+types/docs';
import { DocsLayout } from 'fumadocs-ui/layouts/docs';
import {
  DocsBody,
  DocsDescription,
  DocsPage,
  DocsTitle,
  MarkdownCopyButton,
  ViewOptionsPopover,
} from 'fumadocs-ui/layouts/docs/page';
import { docs, source } from '@/lib/source';
import { baseOptions } from '@/lib/layout.shared';
import { useFumadocsLoader } from 'fumadocs-core/source/client';
import { useMDXComponents } from '@/components/mdx';
import { use } from 'react';
import { appName, getPageImageUrl, getPageMarkdownUrl, gitConfig } from '@/lib/shared';
import { seo } from '@/lib/seo';

export async function loader({ params }: Route.LoaderArgs) {
  const slugs = params['*'].split('/').filter((v) => v.length > 0);
  const page = source.getPage(slugs);
  if (!page) throw new Response('Not found', { status: 404 });

  return {
    title: page.data.title,
    description: page.data.description ?? '',
    url: page.url,
    path: page.path,
    markdownUrl: getPageMarkdownUrl(page).url,
    pageTree: await source.serializePageTree(source.getPageTree()),
    imagePath: getPageImageUrl(page).url,
  };
}

function Content({
  path,
  markdownUrl,
  imagePath,
}: {
  path: string;
  markdownUrl: string;
  imagePath: string;
}) {
  const page = docs.getPage(path);
  if (!page) throw new Error(`unknown page: ${path}`);

  // content is loaded lazily, call `page.preload()` in your loader to avoid suspending
  const { toc } = use(page.load());
  const Mdx = page.body;

  return (
    <DocsPage toc={toc}>
      <DocsTitle>{page.title}</DocsTitle>
      <DocsDescription>{page.description}</DocsDescription>
      <div className="flex flex-row gap-2 items-center border-b -mt-4 pb-6">
        <MarkdownCopyButton markdownUrl={markdownUrl} />
        <ViewOptionsPopover
          markdownUrl={markdownUrl}
          githubUrl={`https://github.com/${gitConfig.user}/${gitConfig.repo}/blob/${gitConfig.branch}/docs/${encodeURI(path)}`}
        />
      </div>
      <DocsBody>
        <Mdx components={useMDXComponents()} />
      </DocsBody>
    </DocsPage>
  );
}

export function meta({ loaderData }: Route.MetaArgs) {
  if (!loaderData) return [{ title: `Page not found | ${appName}` }];
  const { title, description, url, imagePath } = loaderData;
  const fullTitle = url === '/docs' ? `${appName} Docs` : `${title} | ${appName} Docs`;
  return seo({ title: fullTitle, description, path: url, image: imagePath, type: 'article' });
}

export default function Page({ loaderData }: Route.ComponentProps) {
  const { path, pageTree, imagePath, markdownUrl } = useFumadocsLoader(loaderData);

  return (
    <DocsLayout {...baseOptions()} tree={pageTree}>
      <Content path={path} markdownUrl={markdownUrl} imagePath={imagePath} />
    </DocsLayout>
  );
}
