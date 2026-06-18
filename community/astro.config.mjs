import { defineConfig } from 'astro/config';

export default defineConfig({
  site: 'https://pranavagarkar07.github.io',
  base: '/BeamSync',
  outDir: './dist',
  publicDir: './public',
  compressHTML: true,
  build: {
    assets: '_astro',
    inlineStylesheets: 'auto',
  },
});
