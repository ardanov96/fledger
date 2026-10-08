// orval.config.ts - OpenAPI client codegen config for FMCG Wallet.
//
// Sprint 49. Generates lib/api.ts + lib/types.ts from the backend's
// OpenAPI spec (Sprint 49.0 TODO: add OpenAPI annotation middleware to
// cmd/api). For now, the spec is hand-written from docs/api/overview.md.
//
// Usage:
//   pnpm install
//   pnpm typegen        # generates lib/api.ts from openapi.json
//
// Orval docs: https://orval.dev/guides/getting-started
import { defineConfig } from 'orval';

export default defineConfig({
  fmcg: {
    input: {
      target: './openapi.json',
    },
    output: {
      mode: 'tags-split',
      target: './lib/api.ts',
      client: 'fetch',
      baseURL: process.env.NEXT_PUBLIC_API_BASE_URL || 'http://localhost:8080',
      prettier: true,
      clean: true,
      override: {
        operationsGenerator: {
          tags: ['accounts', 'transfers', 'invoices', 'auth', 'notifications'],
        },
      },
    },
    hooks: {
      afterAllFilesWrite: 'prettier --write',
    },
  },
});