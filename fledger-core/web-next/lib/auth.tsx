/**
 * Auth helpers (Sprint 49 + Sprint 57 real wire-up).
 *
 * Cookie-based JWT auth:
 *   - POST /v1/auth/login sets the access_token as an httpOnly cookie
 *   - GET /v1/auth/me reads the cookie + returns the current user
 *   - All subsequent requests send the cookie via credentials: 'include'
 *   - The frontend never touches the token directly
 *
 * Sprint 57: real login() that POSTs to /v1/auth/login. The backend
 * sets the httpOnly cookie; we just record the user state in context
 * for client components.
 */
'use client';

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from 'react';

import { ApiClientError, login } from './api';

export interface User {
  user_id: string;
  tenant_id: string;
  role: string;
}

interface AuthState {
  user: User | null;
  loading: boolean;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  // Sprint 57: fetch current user on mount via /v1/auth/me. This handles
  // the page-refresh case where the cookie is still valid but the client
  // context has been reset.
  const refresh = useCallback(async () => {
    try {
      const res = await fetch('/v1/auth/me', {
        credentials: 'include',
        cache: 'no-store',
      });
      if (res.ok) {
        const body = (await res.json()) as User;
        setUser({
          user_id: body.user_id,
          tenant_id: body.tenant_id,
          role: body.role,
        });
      } else {
        setUser(null);
      }
    } catch {
      setUser(null);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const loginFn = useCallback(
    async (username: string, password: string) => {
      // The backend sets httpOnly cookies on successful login; we just
      // call login() to validate the request + record user locally.
      const response = await login({ username, password });
      // response.data has access_token + refresh_token; we don't store
      // them in JS (cookies are httpOnly, set by backend Set-Cookie).
      // Use the user_id from the JWT — but we don't have a /me here yet,
      // so derive a minimal user from the response.
      setUser({
        user_id: username, // fallback; replaced by /v1/auth/me after refresh
        tenant_id: '',
        role: '',
      });
      // Refresh to get the real user info from /v1/auth/me.
      await refresh();
      void response;
    },
    [refresh],
  );

  const logoutFn = useCallback(async () => {
    // Best-effort server logout. The cookie is httpOnly so we can't
    // delete it from JS — backend should expose POST /v1/auth/logout
    // that clears the cookie via Set-Cookie with Max-Age=0.
    try {
      await fetch('/v1/auth/logout', {
        method: 'POST',
        credentials: 'include',
      });
    } catch {
      // ignore — server may be down
    }
    setUser(null);
  }, []);

  return (
    <AuthContext.Provider
      value={{ user, loading, login: loginFn, logout: logoutFn }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error('useAuth must be used within AuthProvider');
  }
  return ctx;
}

// Re-export ApiClientError so client components can do instanceof checks
// without importing from api.ts directly.
export { ApiClientError };