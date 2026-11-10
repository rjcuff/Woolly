import type { BaseLayoutProps } from 'fumadocs-ui/layouts/shared';
import { SiteThemeToggle, Wordmark } from '@/components/brand';
import { githubUrl } from './shared';

export function baseOptions(): BaseLayoutProps {
  return {
    nav: {
      title: <Wordmark className="text-[30px]" />,
    },
    githubUrl,
    slots: {
      themeSwitch: SiteThemeToggle,
    },
  };
}
