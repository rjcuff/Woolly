import type { ReactNode } from 'react';
import { SiteFooter, SiteHeader } from '@/components/site-chrome';

export function LegalPage({ title, updated, children }: { title: string; updated: string; children: ReactNode }) {
  return (
    <div className="flex min-h-dvh flex-col px-[max(20px,env(safe-area-inset-left))]">
      <SiteHeader />
      <main className="mx-auto w-full max-w-[680px] flex-1 py-12 sm:py-16">
        <h1 className="text-[32px] font-semibold tracking-[-0.02em]">{title}</h1>
        <p className="mt-2 text-sm text-fd-muted-foreground">Last updated {updated}</p>
        <div className="legal mt-10">{children}</div>
      </main>
      <SiteFooter />
    </div>
  );
}
