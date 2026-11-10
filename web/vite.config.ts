import { reactRouter } from '@react-router/dev/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';
import { fumadocsMdx } from 'fumadocs-mdx/vite';

export default defineConfig({
  plugins: [fumadocsMdx(), tailwindcss(), reactRouter()],
  resolve: {
    tsconfigPaths: true,
  },
  // Pre-bundle these up front. Otherwise Vite discovers them mid-session and
  // reloads the page, which can leave an open tab with dead buttons.
  optimizeDeps: {
    include: ['clsx', 'lucide-react', 'motion/react', 'next-themes', 'tailwind-merge'],
  },
});
