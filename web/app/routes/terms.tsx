import type { Route } from './+types/terms';
import { LegalPage } from '@/components/legal-page';
import { seo } from '@/lib/seo';
import { githubUrl } from '@/lib/shared';

export function meta(_: Route.MetaArgs) {
  return seo({
    title: 'Terms | Woolly',
    description: 'The terms for using the Woolly app and website.',
    path: '/terms',
    image: '/og/docs/image.png',
  });
}

export default function Terms() {
  return (
    <LegalPage title="Terms" updated="November 10, 2026">
      <p>By using the Woolly app or this website, you agree to these terms.</p>

      <h2>The software</h2>
      <p>
        Woolly is open source under the <a href={`${githubUrl}/blob/main/LICENSE`}>MIT license</a>.
        You may use, copy, change and share it, including for commercial use, as that license
        allows.
      </p>

      <h2>No warranty</h2>
      <p>
        Woolly is provided as is, without warranty of any kind. You are responsible for your own
        data, including backups. We are not liable for lost data or any damages that come from using
        the software or this website.
      </p>

      <h2>Your data</h2>
      <p>
        Your memories belong to you and stay on your computer. See the <a href="/privacy">privacy
        policy</a> for details.
      </p>

      <h2>Trademark</h2>
      <p>
        Woolly™ and the Woolly logo are trademarks of the Woolly project. The MIT license covers the
        code, not the name or logo. You may say your project works with Woolly or is based on
        Woolly, but please don't use the name or logo in a way that suggests we made or endorse
        your project.
      </p>

      <h2>Changes</h2>
      <p>We may update these terms. If we do, we'll change the date at the top of this page.</p>

      <h2>Contact</h2>
      <p>
        Questions? Open an issue on <a href={githubUrl}>GitHub</a>.
      </p>
    </LegalPage>
  );
}
