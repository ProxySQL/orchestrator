// @ts-check
import { defineConfig } from 'astro/config';
import mdx from '@astrojs/mdx';
import sitemap from '@astrojs/sitemap';

export default defineConfig({
  site: 'https://proxysql.github.io',
  base: '/orchestrator',
  integrations: [mdx(), sitemap()],
});
