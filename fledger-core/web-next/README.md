# FMCG Wallet Web (Sprint 47 — Frontend Migration Plan)

**Status:** Sprint 47 / Fase 6 / Frontend Next.js migration **PLANNING** (Sprint 47.1+ implementation).

This directory is the **migration plan + scaffold** for moving the FMCG
Wallet frontend from vanilla JS (current `web/`) to Next.js 15 + TypeScript.

---

## Why migrate

Current state (`web/` from Sprint 20 / Fase 6 MVP):
- ✅ Zero-dependency vanilla JS SPA — runs without `npm install`
- ✅ Reverse-proxy to API at `/v1/*`
- ❌ No type safety (string-based API calls, manual JSON parsing)
- ❌ No server-side rendering — bad for SEO, slow first paint
- ❌ Manual routing + state management — will not survive a year of feature additions
- ❌ Single-page limits — code-splitting, lazy loading, etc. all manual

Target state (`web-next/` planned):
- ✅ Next.js 15 + App Router
- ✅ TypeScript end-to-end (shared types with backend possible)
- ✅ React Server Components for fast first paint
- ✅ Tailwind + shadcn/ui for fast UI iteration
- ✅ React Query for server-state caching
- ✅ Built-in code-splitting, image optimization, etc.

---

## Migration strategy (incremental, ~2 weeks)

### Sprint 47 — Planning & Scaffold (this sprint)
- ✅ This README documents the migration plan
- ✅ `package.json` scaffold with TypeScript + Next.js + Tailwind
- ✅ Folder structure + tooling config
- ✅ Stub routes for each existing page

### Sprint 47.1 — Type-safe API client
- [ ] Generate TS types from OpenAPI spec (`orval` or `openapi-typescript`)
- [ ] TanStack Query hooks for each endpoint
- [ ] Auth flow (login → JWT in httpOnly cookie)

### Sprint 47.2 — Migrate pages incrementally
- [ ] Sprint 47.2.1 — Login + dashboard layout
- [ ] Sprint 47.2.2 — Accounts list + transfer form
- [ ] Sprint 47.2.3 — Invoices list + payment form
- [ ] Sprint 47.2.4 — Notifications feed
- [ ] Sprint 47.2.5 — Aging report

### Sprint 47.3 — Production hardening
- [ ] E2E tests (Playwright)
- [ ] Lighthouse audit (target score 95+)
- [ ] CSP + security headers
- [ ] Bundle analysis + tree-shaking verification

---

## Folder structure (planned)

```
web-next/
├── app/                      # Next.js App Router
│   ├── (auth)/
│   │   └── login/page.tsx
│   ├── (dashboard)/
│   │   ├── layout.tsx         # shared header/sidebar
│   │   ├── accounts/
│   │   │   ├── page.tsx
│   │   │   └── [id]/page.tsx
│   │   ├── invoices/
│   │   │   ├── page.tsx
│   │   │   └── [id]/page.tsx
│   │   ├── transfers/page.tsx
│   │   ├── notifications/page.tsx
│   │   └── aging/page.tsx
│   ├── api/                   # optional Next.js API routes (if BFF needed)
│   ├── layout.tsx
│   └── page.tsx              # landing page
├── components/
│   ├── ui/                   # shadcn/ui primitives
│   ├── forms/
│   └── layout/
├── lib/
│   ├── api.ts                # generated API client
│   ├── auth.ts                # JWT cookie management
│   └── utils.ts
├── hooks/                     # custom React hooks
├── types/                     # shared TS types
├── public/
├── package.json
├── tsconfig.json
├── tailwind.config.ts
├── next.config.ts
└── README.md (this file)
```

---

## package.json scaffold

```json
{
  "name": "fmcg-wallet-web-next",
  "version": "0.1.0",
  "private": true,
  "scripts": {
    "dev": "next dev -p 3001",
    "build": "next build",
    "start": "next start -p 3001",
    "lint": "next lint",
    "test": "vitest",
    "test:e2e": "playwright test",
    "typegen": "orval --config orval.config.ts"
  },
  "dependencies": {
    "next": "^15.0.0",
    "react": "^19.0.0",
    "react-dom": "^19.0.0",
    "@tanstack/react-query": "^5.0.0",
    "zod": "^3.23.0",
    "tailwindcss": "^3.4.0",
    "lucide-react": "^0.400.0"
  },
  "devDependencies": {
    "typescript": "^5.5.0",
    "@types/node": "^22.0.0",
    "@types/react": "^19.0.0",
    "eslint": "^9.0.0",
    "eslint-config-next": "^15.0.0",
    "orval": "^7.0.0",
    "vitest": "^2.0.0",
    "@playwright/test": "^1.45.0"
  }
}
```

---

## API integration plan

The backend exposes OpenAPI-style endpoints documented in `docs/api/overview.md`.
Sprint 47.1 generates TS types via:

```bash
npx orval --config orval.config.ts
```

which reads `openapi.json` (generated from backend annotations) and emits:
- `lib/api.ts` — typed fetch client (one function per endpoint)
- `lib/types.ts` — request/response schemas

Then React Query hooks wrap each call:

```ts
export const useAccountList = (tenantId: string) =>
  useQuery({
    queryKey: ['accounts', tenantId],
    queryFn: () => api.listAccounts({ tenantId }),
  });
```

---

## Auth flow

Backend uses JWT (Sprint 13/33). Frontend stores JWT in httpOnly cookie
(set by backend on /v1/auth/login). Frontend never touches the token
directly — every API call includes `credentials: 'include'`.

Sprint 47.1 implements:
- `/login` page → POST /v1/auth/login → cookie set → redirect
- `AuthProvider` reads user info from `/v1/auth/me` on mount
- 401 → redirect to login

---

## Migration checklist (for Sprint 47.1+ implementer)

- [ ] Decide: App Router (recommended) vs Pages Router
- [ ] Generate TS types from openapi.json
- [ ] Pick React Query (or SWR if preferred)
- [ ] Pick component lib (shadcn/ui recommended — matches portfolio theme)
- [ ] Pick state lib (Zustand if global state needed)
- [ ] Set up CI for `npm ci && npm run build && npm run lint`
- [ ] Set up Playwright E2E
- [ ] Decide deployment: Vercel (recommended for Next.js) vs self-host (Fly.io)
- [ ] Decide on environment variables: NEXT_PUBLIC_API_BASE_URL
- [ ] Audit accessibility (Lighthouse, axe)
- [ ] Audit performance (Lighthouse score target)

---

## What this sprint (47) DOES NOT do

- ❌ Implement actual pages (Sprint 47.2)
- ❌ Wire API client end-to-end (Sprint 47.1)
- ❌ Set up CI/CD for the new frontend
- ❌ Migrate any existing functionality

This sprint is **planning + scaffold only**. The current `web/`
directory remains the active frontend until Sprint 47.1 ships.

---

## References

- Next.js 15 docs: https://nextjs.org/docs
- shadcn/ui: https://ui.shadcn.com
- TanStack Query: https://tanstack.com/query
- Backend API overview: `docs/api/overview.md`
- Sprint 20 (vanilla JS web frontend): `docs/SPRINTS.md#sprint-20-frontend-dashboard-mvp`