/**
 * Sprint 53: Accounts list page.
 * Migrated from web/public/accounts.html (vanilla JS).
 *
 * Server component (RSC) — fetches data on the server, ships HTML to client.
 * Uses lib/api.ts listAccounts() with the httpOnly JWT cookie.
 */
import { listAccounts } from '@/lib/api';
import { getServerSession } from '@/lib/auth-server';

export const dynamic = 'force-dynamic'; // always fetch fresh

export default async function AccountsPage() {
  const session = await getServerSession();
  if (!session) {
    return (
      <div className="p-8 text-center text-gray-600">
        Please <a href="/login" className="text-brand-600 underline">sign in</a> to view accounts.
      </div>
    );
  }

  let accounts;
  let error: string | null = null;
  try {
    const response = await listAccounts({ tenant_id: session.tenant_id, limit: 100 });
    accounts = response.data;
  } catch (err) {
    error = err instanceof Error ? err.message : 'Failed to load accounts';
    accounts = [];
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Accounts</h1>
        <span className="text-sm text-gray-500">{accounts.length} total</span>
      </div>

      {error && (
        <div className="rounded-md bg-red-50 p-3 mb-4 text-sm text-red-700">{error}</div>
      )}

      <div className="bg-white rounded-lg shadow overflow-hidden">
        <table className="min-w-full divide-y divide-gray-200">
          <thead className="bg-gray-50">
            <tr>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Code</th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Name</th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Type</th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Currency</th>
              <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase">Balance</th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Status</th>
            </tr>
          </thead>
          <tbody className="bg-white divide-y divide-gray-200">
            {accounts.length === 0 ? (
              <tr>
                <td colSpan={6} className="px-6 py-4 text-center text-sm text-gray-500">
                  No accounts yet.
                </td>
              </tr>
            ) : (
              accounts.map((acc) => (
                <tr key={acc.id} className="hover:bg-gray-50">
                  <td className="px-6 py-4 text-sm font-mono">{acc.code}</td>
                  <td className="px-6 py-4 text-sm">{acc.name}</td>
                  <td className="px-6 py-4 text-sm capitalize">{acc.type}</td>
                  <td className="px-6 py-4 text-sm">{acc.currency}</td>
                  <td className="px-6 py-4 text-sm text-right font-mono">
                    {(acc.cached_balance / 100).toLocaleString()} {acc.currency}
                  </td>
                  <td className="px-6 py-4 text-sm">
                    <span
                      className={`inline-flex rounded-full px-2 text-xs font-semibold leading-5 ${
                        acc.status === 'active'
                          ? 'bg-green-100 text-green-800'
                          : 'bg-gray-100 text-gray-800'
                      }`}
                    >
                      {acc.status}
                    </span>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}