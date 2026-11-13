import type { Route } from './+types/home';
import { Link } from 'react-router';
import { SiteFooter, SiteHeader } from '@/components/site-chrome';
import { getStars } from '@/lib/github';
import { integrations } from '@/lib/integrations';
import { seo } from '@/lib/seo';
import { appName, blueskyUrl, githubUrl, linkedinUrl, productHuntBadge, productHuntUrl, siteDescription, tagline } from '@/lib/shared';

export function meta(_: Route.MetaArgs) {
  return [
    ...seo({ title: `${appName} | ${tagline}`, description: siteDescription, path: '/', image: '/og/docs/image.png' }),
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
        sameAs: [githubUrl, blueskyUrl, linkedinUrl],
      },
    },
  ];
}

export async function loader() {
  return { stars: await getStars() };
}

const macDownload = 'https://github.com/rjcuff/woolly/releases/latest/download/Woolly-mac.dmg';

export default function Home({ loaderData }: Route.ComponentProps) {
  return (
    <div className="flex h-dvh flex-col overflow-hidden px-[max(20px,env(safe-area-inset-left))]">
      <SiteHeader stars={loaderData?.stars} />

      <main className="flex flex-1 items-center justify-center pb-[6vh]">
        <section className="flex w-full max-w-[640px] flex-col items-start gap-6">
          <a href={productHuntUrl} target="_blank" rel="noopener noreferrer" className="-mb-1 block transition-opacity hover:opacity-85">
            <img
              src={productHuntBadge('light')}
              alt="Woolly - One local memory for all your AI tools | Product Hunt"
              width={250}
              height={54}
              className="dark:hidden"
            />
            <img
              src={productHuntBadge('dark')}
              alt="Woolly - One local memory for all your AI tools | Product Hunt"
              width={250}
              height={54}
              className="hidden dark:block"
            />
          </a>
          <h1 className="text-balance text-[26px] font-semibold leading-tight tracking-[-0.02em] sm:text-[30px]">
            Memory for your AI, kept on your laptop.
          </h1>
          <p className="text-pretty text-[19px] leading-relaxed sm:text-[22px]">
            Woolly gives Claude, Cursor and your own tools one shared memory. It lives in plain
            files on your machine and never leaves your computer.
          </p>
          <div className="mt-4 flex flex-wrap items-center gap-3">
            <Link
              to="/docs/quickstart"
              className="rounded-full bg-fd-foreground px-8 py-4 text-[18px] font-medium text-fd-background transition-opacity hover:opacity-85"
            >
              Get started
            </Link>
            <a
              href={macDownload}
              className="rounded-full px-6 py-4 text-[18px] font-medium shadow-[0_0_0_1px_var(--color-fd-border)] transition-colors hover:bg-fd-accent"
            >
              Download for Mac
            </a>
          </div>

          <div className="mt-12 w-full [@media(max-height:600px)]:hidden">
            <p className="text-[14px] text-fd-muted-foreground">Integrated with</p>
            <ul className="mt-5 flex flex-wrap items-center gap-x-[18px] gap-y-5 sm:gap-x-9">
              {integrations.map((app) => (
                <li key={app.name} className="group relative">
                  <Link
                    to={app.href}
                    aria-label={app.name}
                    className="block text-fd-muted-foreground transition-colors hover:text-fd-foreground focus-visible:text-fd-foreground"
                  >
                    <svg viewBox="0 0 24 24" className="size-7 sm:size-8" fill="currentColor" aria-hidden="true">
                      <path d={app.path} />
                    </svg>
                  </Link>
                  <span
                    role="tooltip"
                    className="pointer-events-none absolute top-full left-1/2 mt-2 -translate-x-1/2 -translate-y-1 whitespace-nowrap rounded-md bg-fd-foreground px-2 py-1 text-[13px] font-medium text-fd-background opacity-0 transition duration-150 group-hover:translate-y-0 group-hover:opacity-100 group-focus-within:translate-y-0 group-focus-within:opacity-100"
                  >
                    {app.name}
                  </span>
                </li>
              ))}
            </ul>
          </div>
        </section>
      </main>

      <SiteFooter />
    </div>
  );
}
