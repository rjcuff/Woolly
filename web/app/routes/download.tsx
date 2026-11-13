import type { Route } from './+types/download';
import { useEffect, useState } from 'react';
import { SiteFooter, SiteHeader } from '@/components/site-chrome';
import { detectOS, macApp, windowsApp, type OS } from '@/lib/download';
import { getStars } from '@/lib/github';
import { seo } from '@/lib/seo';

export function meta(_: Route.MetaArgs) {
  return seo({
    title: 'Download Woolly',
    description: 'Download Woolly for Mac, Windows and Linux. One local memory for all your AI tools.',
    path: '/download',
    image: '/og/docs/image.png',
  });
}

export async function loader() {
  return { stars: await getStars() };
}

const OSES: { id: OS; label: string }[] = [
  { id: 'mac', label: 'macOS' },
  { id: 'linux', label: 'Linux' },
  { id: 'windows', label: 'Windows' },
];

const COMMANDS: Record<OS, string> = {
  mac: 'curl -fsSL https://usewoolly.com/install.sh | sh',
  linux: 'curl -fsSL https://usewoolly.com/install.sh | sh',
  windows: 'irm https://usewoolly.com/install.ps1 | iex',
};

export default function Download({ loaderData }: Route.ComponentProps) {
  const [os, setOS] = useState<OS>('mac');
  const [copied, setCopied] = useState(false);

  useEffect(() => setOS(detectOS()), []);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(COMMANDS[os]);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1500);
    } catch {
      // Clipboard can be blocked; the command is still there to select.
    }
  };

  return (
    <div className="flex min-h-dvh flex-col px-[max(20px,env(safe-area-inset-left))]">
      <SiteHeader stars={loaderData?.stars} />

      <main className="flex flex-1 items-center justify-center py-12">
        <section className="flex w-full max-w-[560px] flex-col items-center gap-8 text-center">
          <h1 className="text-[30px] font-semibold tracking-[-0.02em] sm:text-[36px]">Download Woolly</h1>

          <div role="tablist" aria-label="Operating system" className="flex rounded-full bg-black/[0.06] p-1 dark:bg-white/10">
            {OSES.map((o) => (
              <button
                key={o.id}
                role="tab"
                aria-selected={os === o.id}
                onClick={() => setOS(o.id)}
                className={`rounded-full px-5 py-2 text-[15px] transition-colors ${
                  os === o.id ? 'bg-fd-background font-medium shadow-sm' : 'text-fd-muted-foreground hover:text-fd-foreground'
                }`}
              >
                {o.label}
              </button>
            ))}
          </div>

          {os === 'mac' && (
            <div className="flex flex-col items-center gap-3">
              <a
                href={macApp}
                className="rounded-full bg-fd-foreground px-8 py-4 text-[18px] font-medium text-fd-background transition-opacity hover:opacity-85"
              >
                Download for Mac
              </a>
              <p className="text-[14px] text-fd-muted-foreground">Apple Silicon and Intel · macOS 11 or later</p>
            </div>
          )}

          {os === 'windows' && (
            <div className="flex flex-col items-center gap-3">
              <a
                href={windowsApp}
                className="rounded-full bg-fd-foreground px-8 py-4 text-[18px] font-medium text-fd-background transition-opacity hover:opacity-85"
              >
                Download for Windows
              </a>
              <p className="text-[14px] text-fd-muted-foreground">Windows 10 or later</p>
            </div>
          )}

          <div className="flex w-full flex-col gap-3">
            <p className="text-[15px] text-fd-muted-foreground">
              {os === 'mac' ? 'Or install the command line tool:' : os === 'windows' ? 'Or install the command line tool in PowerShell:' : 'Paste this in a terminal:'}
            </p>
            <div className="flex items-center gap-2 rounded-2xl bg-black/[0.05] py-2 pr-2 pl-5 text-left dark:bg-white/10">
              <code className="min-w-0 flex-1 overflow-x-auto whitespace-nowrap font-mono text-[14px]">{COMMANDS[os]}</code>
              <button
                onClick={copy}
                className="shrink-0 rounded-full bg-fd-background px-4 py-2 text-[14px] font-medium shadow-sm transition-opacity hover:opacity-80"
              >
                {copied ? 'Copied' : 'Copy'}
              </button>
            </div>
            {os === 'linux' && <p className="text-[14px] text-fd-muted-foreground">Works on x86 and ARM, including Raspberry Pi.</p>}
          </div>

          <p className="text-[14px] text-fd-muted-foreground">
            Then connect your AI apps with <code className="font-mono">woolly connect</code>. See the{' '}
            <a href="/docs/quickstart" className="underline underline-offset-4 hover:text-fd-foreground">
              quickstart
            </a>
            .
          </p>
        </section>
      </main>

      <SiteFooter />
    </div>
  );
}
