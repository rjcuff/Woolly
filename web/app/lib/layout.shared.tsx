import type { BaseLayoutProps } from 'fumadocs-ui/layouts/shared';
import { SiteThemeToggle, Wordmark } from '@/components/brand';
import { githubUrl } from './shared';

export function baseOptions(): BaseLayoutProps {
  return {
    nav: {
      title: <Wordmark className="text-[30px]" />,
    },
    githubUrl,
    links: [
      { text: 'Download', url: '/docs/quickstart', active: 'none' },
      { text: 'Docs', url: '/docs', active: 'nested-url' },
    ],
    slots: {
      themeSwitch: SiteThemeToggle,
    },
  };
}
