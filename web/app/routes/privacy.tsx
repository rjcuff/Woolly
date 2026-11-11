import type { Route } from './+types/privacy';
import { LegalPage } from '@/components/legal-page';
import { seo } from '@/lib/seo';
import { githubUrl } from '@/lib/shared';

export function meta(_: Route.MetaArgs) {
  return seo({
    title: 'Privacy | Woolly',
    description: 'Woolly keeps your memories on your computer. Here is exactly what the app and this website do with data.',
    path: '/privacy',
    image: '/og/docs/image.png',
  });
}

export default function Privacy() {
  return (
    <LegalPage title="Privacy" updated="November 11, 2026">
      <p>
        Woolly is built so your memories stay on your computer. This page explains what the Woolly
        app and this website do with data.
      </p>

      <h2>The Woolly app</h2>
      <ul>
        <li>Your memories are stored as files on your computer, in <code>~/.woolly</code> unless you choose another folder.</li>
        <li>Woolly has no accounts, no analytics and no tracking. It never sends your memories to us or anyone else.</li>
        <li>
          Woolly keeps a log on your computer of what each connected app saved, read and deleted, so you
          can check on them. It's stored in <code>~/.woolly/activity.log</code> and is never sent anywhere.
        </li>
        <li>
          <code>woolly learn</code> sends text to Ollama at the address you set, by default on your own
          computer.
        </li>
        <li>
          If you turn on sync, your memories are sent, encrypted, to the other devices you linked, over
          your own network. They never go to us or any other server.
        </li>
        <li>
          If you turn on search by meaning, Woolly sends text to Ollama at the address you set. By
          default that is Ollama running on your own computer.
        </li>
        <li>
          When you connect Woolly to an AI app such as Claude or Cursor, that app can read and save
          memories. What the app does with that information is covered by its own privacy policy.
        </li>
        <li>
          <code>woolly update</code> contacts GitHub to check for and download a new version, only
          when you run it. GitHub's privacy policy covers that request.
        </li>
        <li>You can delete everything at any time by deleting the <code>~/.woolly</code> folder.</li>
      </ul>

      <h2>This website</h2>
      <ul>
        <li>We don't use cookies or ads.</li>
        <li>
          We use Vercel Web Analytics to count visits. It doesn't use cookies, doesn't identify you
          and doesn't follow you across other sites. It records things like which page you visited,
          the site that linked you here, your country, and your browser and device type.
        </li>
        <li>
          Your light or dark theme choice is saved in your browser's local storage. It never leaves
          your browser.
        </li>
        <li>
          The site is hosted by Vercel, which keeps standard server logs such as IP address, browser
          type and pages requested. See Vercel's privacy policy for details.
        </li>
        <li>Fonts are loaded from Google Fonts, which receives your IP address when the fonts load.</li>
        <li>The install script downloads Woolly from GitHub, which is covered by GitHub's privacy policy.</li>
      </ul>

      <h2>Children</h2>
      <p>Woolly is not directed at children under 13, and we don't knowingly collect their information.</p>

      <h2>Changes</h2>
      <p>If this policy changes, we'll update this page and the date at the top.</p>

      <h2>Contact</h2>
      <p>
        Questions? Open an issue on <a href={githubUrl}>GitHub</a>.
      </p>
    </LegalPage>
  );
}
