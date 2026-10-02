/**
 * Sprint 50: Server-side session helper.
 *
 * Reads the httpOnly JWT cookie set by POST /v1/auth/login. Used by
 * server components (RSC) for SSR user info without a client fetch.
 *
 * Sprint 49.1+: replace stub with real /v1/auth/me call to backend.
 */
import 'server-only';

import type { User } from './auth';

export async function getServerSession(): Promise<User | null> {
  // Sprint 49.1: read JWT cookie via next/headers cookies(),
  // call /v1/auth/me with the token, return User or null.
  return null;
}