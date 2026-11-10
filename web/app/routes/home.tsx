import type { Route } from './+types/home';
import { Link } from 'react-router';
import { GitHubIcon, SiteThemeToggle, Wordmark } from '@/components/brand';
import { seo } from '@/lib/seo';
import { appName, githubUrl, siteDescription, tagline } from '@/lib/shared';

export function meta(_: Route.MetaArgs) {
  return [
    ...seo({ title: `${appName} | ${tagline}`, description: siteDescription, path: '/', image: '/og/docs/image.webp' }),
    {
      'script:ld+json': {
        '@context': 'https://schema.org',
        '@type': 'SoftwareApplication',
        name: appName,
        description: siteDescription,
        applicationCategory: 'DeveloperApplication',
        operatingSystem: 'macOS, Linux, Windows',
        offers: { '@type': 'Offer', price: '0', priceCurrency: 'USD' },
        license: 'https://opensource.org/licenses/MIT',
      },
    },
  ];
}

export default function Home() {
  return (
    <div className="flex h-dvh flex-col overflow-hidden px-[max(20px,env(safe-area-inset-left))]">
      <header className="flex items-center justify-between gap-6 py-5">
        <Link to="/" aria-label="Woolly home">
          <Wordmark className="text-[28px] sm:text-[34px]" />
        </Link>
        <nav aria-label="Main" className="flex items-center gap-4 text-[16px] sm:gap-8 sm:text-lg">
          <a href={githubUrl} aria-label="GitHub" className="hover:text-fd-muted-foreground">
            <GitHubIcon className="size-5 sm:hidden" />
            <span className="max-sm:hidden">GitHub</span>
          </a>
          <Link to="/docs/quickstart" className="hover:text-fd-muted-foreground">
            Download
          </Link>
          <Link to="/docs" className="hover:text-fd-muted-foreground">
            Docs
          </Link>
          <SiteThemeToggle className="-ml-2 sm:-ml-3" />
        </nav>
      </header>

      <main className="flex flex-1 items-center justify-center pb-[8vh]">
        <section className="flex w-full max-w-[640px] flex-col items-start gap-6">
          <h1 className="text-balance text-[26px] font-semibold leading-tight tracking-[-0.02em] sm:text-[30px]">
            Memory for your AI, kept on your laptop.
          </h1>
          <p className="text-pretty text-[19px] leading-relaxed sm:text-[22px]">
            Woolly gives Claude, Cursor and your own tools one shared memory. It lives in plain
            files on your machine and never leaves your computer.
          </p>
          <Link
            to="/docs/quickstart"
            className="mt-4 rounded-full bg-fd-foreground px-8 py-4 text-[18px] font-medium text-fd-background transition-opacity hover:opacity-85"
          >
            Get started
          </Link>
        </section>
      </main>
    </div>
  );
}
