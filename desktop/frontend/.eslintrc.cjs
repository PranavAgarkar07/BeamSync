module.exports = {
  root: true,
  extends: ['eslint:recommended'],
  plugins: ['svelte3'],
  overrides: [{ files: ['*.svelte'], processor: 'svelte3/svelte3' }],
  env: { browser: true, node: true, es2022: true },
  parserOptions: { ecmaVersion: 'latest', sourceType: 'module' },
};
