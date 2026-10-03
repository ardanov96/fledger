/**
 * Sprint 55: Invoices list page.
 * Migrated from web/public/invoices.html (vanilla JS).
 */
import { listInvoices } from '@/lib/api';
import { getServerSession } from '@/lib/auth-server';

export const dynamic = 'force-dynamic';

export default async function InvoicesPage() {
  const session = await getServerSession();
  if (!session) {
    return (
      <div className="p-8 text-center text-gray-600">
        Please <a href="/login" className="text-brand-600 underline">sign in</a> to view invoices.
      </div>
    );
  }

  let invoices;
  let error: string | null = null;
  try {
    const response = await listInvoices({ tenant_id: session.tenant_id });
    invoices = response.data;
  } catch (err) {
    error = err instanceof Error ? err.message : 'Failed to load invoices';
    invoices = [];
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Invoices</h1>
        <span className="text-sm text-gray-500">{invoices.length} total</span>
      </div>

      {error && (
        <div className="rounded-md bg-red-50 p-3 mb-4 text-sm text-red-700">{error}</div>
      )}

      <div className="bg-white rounded-lg shadow overflow-hidden">
        <table className="min-w-full divide-y divide-gray-200">
          <thead className="bg-gray-50">
            <tr>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Code</th>
              <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase">Amount</th>
              <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase">Paid</th>
              <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase">Outstanding</th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Status</th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Due</th>
            </tr>
          </thead>
          <tbody className="bg-white divide-y divide-gray-200">
            {invoices.length === 0 ? (
              <tr>
                <td colSpan={6} className="px-6 py-4 text-center text-sm text-gray-500">
                  No invoices yet.
                </td>
              </tr>
            ) : (
              invoices.map((inv) => (
                <tr key={inv.id} className="hover:bg-gray-50">
                  <td className="px-6 py-4 text-sm font-mono">{inv.code}</td>
                  <td className="px-6 py-4 text-sm text-right font-mono">
                    {(inv.amount / 100).toLocaleString()}
                  </td>
                  <td className="px-6 py-4 text-sm text-right font-mono text-green-700">
                    {(inv.paid_amount / 100).toLocaleString()}
                  </td>
                  <td className="px-6 py-4 text-sm text-right font-mono">
                    {((inv.amount - inv.paid_amount) / 100).toLocaleString()}
                  </td>
                  <td className="px-6 py-4 text-sm">
                    <span
                      className={`inline-flex rounded-full px-2 text-xs font-semibold ${
                        inv.status === 'paid'
                          ? 'bg-green-100 text-green-800'
                          : inv.status === 'overdue'
                            ? 'bg-red-100 text-red-800'
                            : inv.status === 'partial'
                              ? 'bg-yellow-100 text-yellow-800'
                              : 'bg-gray-100 text-gray-800'
                      }`}
                    >
                      {inv.status}
                    </span>
                  </td>
                  <td className="px-6 py-4 text-sm text-gray-500">
                    {new Date(inv.due_date).toLocaleDateString()}
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