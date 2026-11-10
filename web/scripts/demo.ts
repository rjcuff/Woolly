// Records the README demo GIF from real Woolly output.
//
//   make demo
//
// It runs each command with the woolly binary in a throwaway folder, draws a
// terminal frame by frame in a headless browser, and hands the frames to
// scripts/gifenc to build docs/demo.gif.
import { execFileSync } from 'node:child_process';
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { chromium } from '@playwright/test';

const root = resolve(import.meta.dirname, '../..');
const bin = join(root, 'bin/woolly');
const home = mkdtempSync(join(tmpdir(), 'woolly-demo-'));
const frames = mkdtempSync(join(tmpdir(), 'woolly-frames-'));

type Step = { cmd: string; args: string[]; pause: number; tidy?: (out: string) => string };

// IDs and dates make lines long; drop them so the demo reads at a glance.
const short = (out: string) => out.replace(/\[\d{8}-\d{6}-[0-9a-f]{8}\] /g, '').replace(/; \d{4}-\d{2}-\d{2}\)/g, ')');

const steps: Step[] = [
  { cmd: 'woolly remember "I live in Austin" --key user.location', args: ['remember', 'I live in Austin', '--key', 'user.location'], pause: 120, tidy: short },
  { cmd: 'woolly remember "Moved to Denver in October" --key user.location', args: ['remember', 'Moved to Denver in October', '--key', 'user.location'], pause: 200, tidy: short },
  { cmd: 'woolly remember "I prefer short answers"', args: ['remember', 'I prefer short answers'], pause: 120, tidy: short },
  { cmd: 'woolly recall "where do I live"', args: ['recall', 'where do I live'], pause: 220, tidy: (o) => short(o).replace(/\n\(keyword search.*\)\n?/, '\n') },
  { cmd: 'woolly remember "my OpenAI key is sk-proj-a8F3kQ9zLm2..."', args: ['remember', 'my OpenAI key is sk-proj-a8F3kQ9zLm2xR7tV4bN6cW1yE5'], pause: 240 },
  { cmd: 'woolly profile', args: ['profile'], pause: 400 },
];

function run(args: string[]): string {
  try {
    return execFileSync(bin, args, {
      env: { ...process.env, WOOLLY_HOME: home, WOOLLY_NO_EMBED: '1' },
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'pipe'],
    });
  } catch (e) {
    const err = e as { stdout?: string; stderr?: string };
    return (err.stdout ?? '') + (err.stderr ?? '');
  }
}

const escape = (s: string) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');

function colorize(line: string): string {
  const l = escape(line);
  if (l.startsWith('woolly: not saved')) return `<span class="red">${l}</span>`;
  if (l.startsWith('Replaced older memory')) return `<span class="amber">${l}</span>`;
  if (l.startsWith('Remembered:')) return `<span class="green">Remembered:</span>${l.slice('Remembered:'.length)}`;
  if (l.startsWith('#')) return `<span class="bold">${l}</span>`;
  return l;
}

const page = (body: string) => `<!doctype html><html><head><style>
  body { margin: 0; background: #0e0e0e; }
  .win { width: 1000px; height: 560px; background: #0e0e0e; color: #f2f2f2; box-sizing: border-box;
         font: 17px/1.55 "SF Mono", Menlo, monospace; padding: 0; overflow: hidden; }
  .bar { height: 36px; background: #2a2a2a; display: flex; align-items: center; gap: 8px; padding: 0 14px; }
  .dot { width: 12px; height: 12px; border-radius: 50%; background: #8a8a8a; }
  .term { padding: 18px 24px; white-space: pre-wrap; }
  .p { color: #6cc28f; } .green { color: #6cc28f; } .red { color: #f07167; } .amber { color: #e0b05a; }
  .muted { color: #8a8a8a; } .bold { font-weight: 700; }
  .cursor { display: inline-block; width: 10px; height: 20px; background: #f2f2f2; vertical-align: -4px; }
</style></head><body><div class="win"><div class="bar"><span class="dot"></span><span class="dot"></span><span class="dot"></span></div><div class="term">${body}</div></div></body></html>`;

async function main() {
  const browser = await chromium.launch();
  const tab = await browser.newPage({ viewport: { width: 1000, height: 560 }, deviceScaleFactor: 1 });
  const durations: number[] = [];
  let n = 0;
  let history: string[] = [];

  const shot = async (body: string, cs: number) => {
    await tab.setContent(page(body));
    await tab.locator('.win').screenshot({ path: join(frames, `${String(n++).padStart(4, '0')}.png`) });
    durations.push(cs);
  };
  // Keep only the lines that fit on screen.
  const view = (lines: string[]) => lines.slice(-17).join('\n');

  await shot(view(['<span class="p">$</span> <span class="cursor"></span>']), 80);
  for (const step of steps) {
    const output = (step.tidy ?? ((o) => o))(run(step.args)).trimEnd();
    for (let i = 2; i <= step.cmd.length + 1; i += 2) {
      const typed = escape(step.cmd.slice(0, i));
      await shot(view([...history, `<span class="p">$</span> ${typed}<span class="cursor"></span>`]), 4);
    }
    history.push(`<span class="p">$</span> ${escape(step.cmd)}`);
    history.push(...output.split('\n').map(colorize));
    history.push('');
    await shot(view([...history, '<span class="p">$</span> <span class="cursor"></span>']), step.pause);
    if (step.cmd === 'woolly recall "where do I live"') history = [];
  }

  await browser.close();
  writeFileSync(join(frames, 'durations.json'), JSON.stringify(durations));
  const out = join(root, 'docs/demo.gif');
  mkdirSync(join(root, 'docs'), { recursive: true });
  execFileSync('go', ['run', './scripts/gifenc', frames, out], { cwd: root, stdio: 'inherit' });
  rmSync(home, { recursive: true, force: true });
  rmSync(frames, { recursive: true, force: true });
}

await main();
