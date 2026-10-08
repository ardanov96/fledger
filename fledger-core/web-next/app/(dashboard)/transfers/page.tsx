/**
 * Sprint 54: Transfers list + new transfer form.
 * Migrated from web/public/transfers.html (vanilla JS).
 *
 * Layout: list of recent transfers + collapsible form to create new.
 * The form uses a client component so it can manage local state.
 */
import { listTransfers } from '@/lib/api';
import { getServerSession } from '@/lib/auth-server';
import { NewTransferForm } from './new-transfer-form';

export const dynamic = 'force-dynamic';

export default async function TransfersPage() {
  const session = await getServerSession();
  if (!session) {
    return (
      <div className="p-8 text-center text-gray-600">
        Please <a href="/login" className="text-brand-600 underline">sign in</a> to view transfers.
      </div>
    );
  }

  let transfers;
  let error: string | null = null;
  try {
    const response = await listTransfers({ tenant_id: session.tenant_id, limit: 50 });
    transfers = response.data;
  } catch (err) {
    error = err instanceof Error ? err.message : 'Failed to load transfers';
    transfers = [];
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Transfers</h1>
        <span className="text-sm text-gray-500">{transfers.length} recent</span>
      </div>

      {error && (
        <div className="rounded-md bg-red-50 p-3 mb-4 text-sm text-red-700">{error}</div>
      )}

      {/* New transfer form */}
      <details className="bg-white rounded-lg shadow mb-6">
        <summary className="px-6 py-3 cursor-pointer text-sm font-medium text-gray-700 hover:bg-gray-50">
          + New transfer
        </summary>
        <div className="px-6 pb-6">
          <NewTransferForm tenantId={session.tenant_id} />
        </div>
      </details>

      {/* Transfers list */}
      <div className="bg-white rounded-lg shadow overflow-hidden">
        <table className="min-w-full divide-y divide-gray-200">
          <thead className="bg-gray-50">
            <tr>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">From</th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">To</th>
              <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase">Amount</th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Status</th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">When</th>
            </tr>
          </thead>
          <tbody className="bg-white divide-y divide-gray-200">
            {transfers.length === 0 ? (
              <tr>
                <td colSpan={5} className="px-6 py-4 text-center text-sm text-gray-500">
                  No transfers yet.
                </td>
              </tr>
            ) : (
              transfers.map((t) => (
                <tr key={t.id} className="hover:bg-gray-50">
                  <td className="px-6 py-4 text-sm font-mono">
                    {t.from_account_id.slice(0, 8)}…
                  </td>
                  <td className="px-6 py-4 text-sm font-mono">
                    {t.to_account_id.slice(0, 8)}…
                  </td>
                  <td className="px-6 py-4 text-sm text-right font-mono">
                    {(t.amount / 100).toLocaleString()} {t.currency}
                  </td>
                  <td className="px-6 py-4 text-sm">
                    <span
                      className={`inline-flex rounded-full px-2 text-xs font-semibold ${
                        t.status === 'posted'
                          ? 'bg-green-100 text-green-800'
                          : t.status === 'failed'
                            ? 'bg-red-100 text-red-800'
                            : 'bg-yellow-100 text-yellow-800'
                      }`}
                    >
                      {t.status}
                    </span>
                  </td>
                  <td className="px-6 py-4 text-sm text-gray-500">
                    {new Date(t.created_at).toLocaleString()}
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