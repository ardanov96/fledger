/**
 * Hand-written API client (Sprint 49 stopgap).
 *
 * The full Sprint 49.1 plan is to run `orval` to generate this file
 * from openapi.json. For now, this file documents the API surface
 * using manually written fetcher functions. When the codegen pipeline
 * is wired (CI step + `pnpm typegen`), replace this with the generated
 * lib/api.ts.
 *
 * Each function:
 *   1. Sets Authorization header from cookie (handled by browser)
 *   2. Fetches from NEXT_PUBLIC_API_BASE_URL + path
 *   3. Throws on non-2xx with structured Error
 */

export interface ApiError {
  code: string;
  message: string;
}

export class ApiClientError extends Error {
  constructor(public status: number, public apiError: ApiError) {
    super(`${status}: ${apiError.code} - ${apiError.message}`);
    this.name = 'ApiClientError';
  }
}

function getBaseUrl(): string {
  return process.env.NEXT_PUBLIC_API_BASE_URL || 'http://localhost:8080';
}

async function request<T>(
  method: string,
  path: string,
  body?: unknown,
  init?: RequestInit,
): Promise<T> {
  const baseUrl = getBaseUrl();
  const url = `${baseUrl}${path}`;
  const res = await fetch(url, {
    method,
    credentials: 'include', // send httpOnly cookie
    headers: {
      'Content-Type': 'application/json',
      ...init?.headers,
    },
    body: body ? JSON.stringify(body) : undefined,
    ...init,
  });
  if (!res.ok) {
    let apiErr: ApiError = { code: 'UNKNOWN', message: res.statusText };
    try {
      apiErr = await res.json();
    } catch {
      /* ignore */
    }
    throw new ApiClientError(res.status, apiErr);
  }
  return res.json() as Promise<T>;
}

// =============================================================================
// Auth (Sprint 13 + 33)
// =============================================================================

export interface LoginRequest {
  username: string;
  password: string;
}

export interface LoginResponse {
  data: {
    access_token: string;
    refresh_token: string;
    expires_at: string;
  };
}

export const login = (req: LoginRequest) =>
  request<LoginResponse>('POST', '/v1/auth/login', req);

// =============================================================================
// Accounts (Sprint 1-2)
// =============================================================================

export interface Account {
  id: string;
  code: string;
  name: string;
  type: 'asset' | 'liability' | 'equity' | 'revenue' | 'expense';
  status: 'active' | 'suspended' | 'closed';
  currency: string;
  cached_balance: number;
  tenant_id: string;
  created_at: string;
}

export const listAccounts = (params: { tenant_id: string; type?: string; limit?: number }) => {
  const search = new URLSearchParams();
  search.set('tenant_id', params.tenant_id);
  if (params.type) search.set('type', params.type);
  if (params.limit) search.set('limit', String(params.limit));
  return request<{ data: Account[] }>('GET', `/v1/accounts?${search}`);
};

// =============================================================================
// Transfers (Sprint 4)
// =============================================================================

export interface Transfer {
  id: string;
  from_account_id: string;
  to_account_id: string;
  amount: number;
  currency: string;
  status: 'pending' | 'posted' | 'reversed' | 'failed';
  created_at: string;
}

export const listTransfers = (params: { tenant_id: string; limit?: number }) => {
  const search = new URLSearchParams();
  search.set('tenant_id', params.tenant_id);
  if (params.limit) search.set('limit', String(params.limit));
  return request<{ data: Transfer[] }>('GET', `/v1/transfers?${search}`);
};

// =============================================================================
// Invoices (Sprint 6)
// =============================================================================

export interface Invoice {
  id: string;
  code: string;
  customer_id: string;
  amount: number;
  paid_amount: number;
  status: 'open' | 'partial' | 'paid' | 'overdue';
  due_date: string;
}

export const listInvoices = (params: { tenant_id: string; status?: string }) => {
  const search = new URLSearchParams();
  search.set('tenant_id', params.tenant_id);
  if (params.status) search.set('status', params.status);
  return request<{ data: Invoice[] }>('GET', `/v1/invoices?${search}`);
};

// =============================================================================
// Notifications (Sprint 28)
// =============================================================================

export interface Notification {
  id: string;
  type: string;
  title: string;
  body: Record<string, unknown>;
  severity: 'info' | 'warn' | 'critical';
  status: 'unread' | 'read' | 'archived';
  created_at: string;
}

export const listNotifications = (params: { unread?: boolean; limit?: number }) => {
  const search = new URLSearchParams();
  if (params.unread) search.set('unread', 'true');
  if (params.limit) search.set('limit', String(params.limit));
  return request<{ data: Notification[] }>('GET', `/v1/notifications?${search}`);
};

// markNotificationRead (Sprint 55) flips status from 'unread' to 'read'.
// Returns void on success; the handler doesn't return a body.
export const markNotificationRead = (id: string) =>
  request<{ status: 'read' }>('PATCH', `/v1/notifications/${id}/read`);