/**
 * Sprint 57 (real): Server-side session helper.
 *
 * Reads the httpOnly JWT cookie set by POST /v1/auth/login, calls the
 * backend's GET /v1/auth/me to validate it, and returns the User.
 *
 * Used by server components (RSC) for SSR user info without a client fetch.
 * On 401, returns null (caller renders /login link).
 */
import 'server-only';

import { cookies } from 'next/headers';

import { ApiClientError } from './api';
import type { User } from './auth';

const BACKEND_BASE_URL =
  process.env.NEXT_PUBLIC_API_BASE_URL || 'http://localhost:8080';

interface MeResponse {
  user_id: string;
  tenant_id: string;
  role: string;
  scopes?: string[];
}

/**
 * GetServerSession reads the httpOnly JWT cookie (set by Next.js's
 * reverse proxy + the backend's POST /v1/auth/login), forwards it to
 * GET /v1/auth/me, and returns the authenticated User.
 *
 * Returns null on any failure: missing cookie, expired token, backend
 * unreachable, or non-2xx response. The caller should render the
 * "please sign in" UI.
 *
 * Sprint 57 implementation notes:
 *   - Uses `cookies()` from next/headers (App Router server API).
 *   - JWT_COOKIE_NAME matches the cookie name set by the backend on login.
 *     Backend currently sets cookie via standard Set-Cookie header
 *     (default name `token` for refresh, `access_token` for access).
 *     For httpOnly cookie auth, we use `access_token` (already set as
 *     httpOnly in cmd/api/main.go).
 *   - The cookie is forwarded as a Bearer header to /v1/auth/me.
 *     (Alternatively, we could configure Next.js to forward all cookies
 *     via next.config.js `rewrites` — but for /v1/auth/me the backend
 *     middleware reads the Bearer header, not cookies, so this manual
 *     forward is the cleanest path.)
 */
export async function getServerSession(): Promise<User | null> {
  try {
    const cookieStore = await cookies();
    const accessToken = cookieStore.get('access_token')?.value;
    if (!accessToken) {
      return null;
    }

    const res = await fetch(`${BACKEND_BASE_URL}/v1/auth/me`, {
      method: 'GET',
      headers: {
        Authorization: `Bearer ${accessToken}`,
        Accept: 'application/json',
      },
      // Server-side fetch: no cookies to forward (we're using Bearer).
      cache: 'no-store', // never cache auth state
    });

    if (!res.ok) {
      // 401 → token expired/invalid; 5xx → backend down.
      // Both cases: caller should re-prompt for login.
      return null;
    }

    const body = (await res.json()) as MeResponse;
    return {
      user_id: body.user_id,
      tenant_id: body.tenant_id,
      role: body.role,
    };
  } catch {
    // Network error, JSON parse error, etc.
    return null;
  }
}