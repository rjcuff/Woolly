import { gitConfig } from './shared';

// Show the star count only once it's big enough to help. Below this the nav
// just says "GitHub".
export const STAR_THRESHOLD = 100;

// getStars reads the repo's star count. It runs when the site is built, so
// it's as fresh as the latest deploy, and visitors' browsers never call GitHub.
export async function getStars(): Promise<number | null> {
  try {
    const res = await fetch(`https://api.github.com/repos/${gitConfig.user}/${gitConfig.repo}`, {
      headers: { Accept: 'application/vnd.github+json' },
      signal: AbortSignal.timeout(5000),
    });
    if (!res.ok) return null;
    const data = (await res.json()) as { stargazers_count?: number };
    return typeof data.stargazers_count === 'number' ? data.stargazers_count : null;
  } catch {
    return null;
  }
}

// formatStars turns 186700 into "186.7K" and 1234 into "1.2K".
export function formatStars(n: number): string {
  if (n >= 1_000_000) return `${trim(n / 1_000_000)}M`;
  if (n >= 1_000) return `${trim(n / 1_000)}K`;
  return String(n);
}

function trim(x: number): string {
  return x.toFixed(1).replace(/\.0$/, '');
}
