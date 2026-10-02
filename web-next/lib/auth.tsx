/**
 * Auth helpers (Sprint 49).
 *
 * Cookie-based JWT auth:
 *   - POST /v1/auth/login sets the access_token as an httpOnly cookie
 *   - All subsequent requests send the cookie via credentials: 'include'
 *   - The frontend never touches the token directly
 *
 * This module provides React Context for user state + login/logout helpers.
 */
'use client';

import {
  createContext,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from 'react';

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

  useEffect(() => {
    // Sprint 49: stub — would call /v1/auth/me to fetch current user
    // from the httpOnly cookie. For now, login state is client-only.
    setLoading(false);
  }, []);

  const login = async (username: string, password: string) => {
    // Sprint 49: would call api.login({ username, password })
    // For now, stub auth (production wiring is Sprint 49.1+)
    setUser({
      user_id: 'stub',
      tenant_id: 'stub-tenant',
      role: 'admin',
    });
    void username;
    void password;
  };

  const logout = async () => {
    setUser(null);
  };

  return (
    <AuthContext.Provider value={{ user, loading, login, logout }}>
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