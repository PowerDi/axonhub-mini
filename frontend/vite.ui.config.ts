import { defineConfig, mergeConfig } from 'vite';
import config from './vite.config.ts';

// UI regressions must not invalidate the managed dev server's dependency cache.
export default mergeConfig(config, defineConfig({ cacheDir: 'node_modules/.vite-ui-regression' }));
