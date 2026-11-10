import { readFile } from 'node:fs/promises';
import { join } from 'node:path';
import { Renderer } from '@takumi-rs/core';
import { ImageResponse } from 'takumi-js/response';

// Link previews: "Woolly" in the wordmark font, centered, with one line under it.
// Images are rendered at build time, so the font files are read from disk.

let renderer: Promise<Renderer> | undefined;

function getRenderer(): Promise<Renderer> {
  renderer ??= (async () => {
    const dir = join(process.cwd(), 'app/og/fonts');
    const r = new Renderer();
    await r.registerFont({ name: 'Shrikhand', data: await readFile(join(dir, 'Shrikhand-Regular.ttf')) });
    await r.registerFont({ name: 'Geist', data: await readFile(join(dir, 'Geist.ttf')) });
    return r;
  })();
  return renderer;
}

export async function ogImage(subtitle: string): Promise<Response> {
  return new ImageResponse(
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        width: '100%',
        height: '100%',
        backgroundColor: '#f4f4f2',
        color: '#111111',
        fontFamily: 'Geist',
      }}
    >
      <div style={{ fontFamily: 'Shrikhand', fontSize: 210, lineHeight: 1, transform: 'rotate(-2deg)' }}>
        Woolly
      </div>
      <div style={{ marginTop: 44, fontSize: 44, color: '#555555', fontWeight: 500 }}>{subtitle}</div>
    </div>,
    { width: 1200, height: 630, format: 'png', renderer: await getRenderer() },
  );
}
