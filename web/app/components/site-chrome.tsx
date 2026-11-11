import { Link } from 'react-router';
import { BlueskyIcon, GitHubIcon, LinkedInIcon, SiteThemeToggle, Wordmark } from '@/components/brand';
import { formatStars, STAR_THRESHOLD } from '@/lib/github';
import { blueskyUrl, githubUrl, linkedinUrl } from '@/lib/shared';

export function SiteHeader({ stars }: { stars?: number | null }) {
  const showStars = typeof stars === 'number' && stars >= STAR_THRESHOLD;
  return (
    <header className="flex items-center justify-between gap-6 py-5">
      <Link to="/" aria-label="Woolly home">
        <Wordmark className="text-[28px] sm:text-[34px]" />
      </Link>
      <nav aria-label="Main" className="flex items-center gap-4 text-[16px] sm:gap-8 sm:text-lg">
        <a
          href={githubUrl}
          aria-label={showStars ? `GitHub, ${stars} stars` : 'GitHub'}
          className="flex items-center gap-2 hover:text-fd-muted-foreground"
        >
          <GitHubIcon className={showStars ? 'size-5' : 'size-5 sm:hidden'} />
          {showStars ? (
            <span className="tabular-nums">{formatStars(stars)}</span>
          ) : (
            <span className="max-sm:hidden">GitHub</span>
          )}
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
  );
}

export function SiteFooter() {
  const year = new Date().getFullYear();
  return (
    <footer className="flex flex-wrap items-center justify-between gap-x-6 gap-y-2 py-5 text-xs text-fd-muted-foreground">
      <span>© {year} Woolly™</span>
      <nav aria-label="Footer" className="flex items-center gap-5">
        <Link to="/privacy" className="hover:text-fd-foreground">
          Privacy
        </Link>
        <Link to="/terms" className="hover:text-fd-foreground">
          Terms
        </Link>
        <a href={githubUrl} className="hover:text-fd-foreground">
          GitHub
        </a>
        <a href={blueskyUrl} aria-label="Bluesky" title="Woolly on Bluesky" className="hover:text-fd-foreground">
          <BlueskyIcon className="size-4" />
        </a>
        <a href={linkedinUrl} aria-label="LinkedIn" title="Woolly on LinkedIn" className="hover:text-fd-foreground">
          <LinkedInIcon className="size-[15px]" />
        </a>
      </nav>
    </footer>
  );
}
