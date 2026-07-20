# Phase 2: Tooling & DevEx

## Requirements

| ID | Description | Type |
|----|------------|------|
| TOL-01 | golangci-lint for `beamsync/` module (govet, staticcheck, errcheck, gosec, errorlint) | Config |
| TOL-02 | golangci-lint for `desktop/` module | Config |
| TOL-03 | ESLint + eslint-plugin-svelte3 for `desktop/frontend/` | Config |
| TOL-04 | Prettier config for `desktop/frontend/` | Config |
| TOL-05 | Scaffold Vitest in `desktop/frontend/` | Config |
| TOL-06 | svelte-check for frontend type checking | Config |
| TOL-07 | GitHub Actions CI: lint → test → build (per-module) | Config |

## Decisions

All decisions follow standard conventions with zero custom configuration:

1. **golangci-lint**: Separate `.golangci.yml` in `beamsync/` and `desktop/`. Linters: `govet`, `staticcheck`, `errcheck`, `gosec`, `errorlint`.
2. **ESLint**: Legacy `.eslintrc.cjs` format (compatible with eslint-plugin-svelte3). Standard rules.
3. **Prettier**: `.prettierrc` with `singleQuote: true`, `trailingComma: 'all'`, `plugins: ['prettier-plugin-svelte']`.
4. **Vitest**: `vitest.config.ts` with jsdom environment, `@testing-library/svelte`.
5. **svelte-check**: Add as devDependency with npm script.
6. **CI**: Single `.github/workflows/ci.yml` with matrix strategy covering lint-go, lint-svelte, test-go, test-svelte, build.

## Files Created

| File | Tool |
|------|------|
| `beamsync/.golangci.yml` | golangci-lint |
| `desktop/.golangci.yml` | golangci-lint |
| `desktop/frontend/.eslintrc.cjs` | ESLint |
| `desktop/frontend/.prettierrc` | Prettier |
| `desktop/frontend/vitest.config.ts` | Vitest |
| `.github/workflows/ci.yml` | GitHub Actions |

## Package Dependencies Added

```json
{
  "devDependencies": {
    "eslint": "^8.0.0",
    "eslint-plugin-svelte3": "^4.0.0",
    "prettier": "^3.0.0",
    "prettier-plugin-svelte": "^3.0.0",
    "vitest": "^1.0.0",
    "@testing-library/svelte": "^4.0.0",
    "jsdom": "^24.0.0",
    "svelte-check": "^3.0.0"
  }
}
```
