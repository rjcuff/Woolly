import { ThemeToggle } from '@/components/motion/theme-toggle';
import { cn } from '@/lib/utils';

export function Wordmark({ className }: { className?: string }) {
  return <span className={cn('wordmark', className)}>Woolly</span>;
}

// The easeUI toggle, sized to sit in Fumadocs' nav and sidebar.
export function SiteThemeToggle({ className }: { className?: string }) {
  return (
    <ThemeToggle
      variant="ripple"
      className={cn(
        'size-9 rounded-full text-fd-muted-foreground transition-colors hover:bg-fd-accent hover:text-fd-accent-foreground',
        className,
      )}
      iconClassName="size-[18px]"
    />
  );
}
