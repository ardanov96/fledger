/**
 * Sprint 50: Dashboard overview page.
 * Server component (RSC) — fetches data on the server, ships HTML to client.
 *
 * Sprint 49.1: wire to /v1/notifications for the recent-activity widget.
 */
export default async function DashboardPage() {
  return (
    <div>
      <h1 className="text-2xl font-bold mb-6">Dashboard</h1>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div className="bg-white rounded-lg shadow p-6">
          <h2 className="text-sm font-medium text-gray-500">Recent Transfers</h2>
          <p className="mt-2 text-3xl font-semibold">—</p>
          <p className="mt-1 text-xs text-gray-400">
            Sprint 50: not yet wired to /v1/transfers
          </p>
        </div>
        <div className="bg-white rounded-lg shadow p-6">
          <h2 className="text-sm font-medium text-gray-500">Outstanding Invoices</h2>
          <p className="mt-2 text-3xl font-semibold">—</p>
          <p className="mt-1 text-xs text-gray-400">
            Sprint 50: not yet wired to /v1/invoices
          </p>
        </div>
        <div className="bg-white rounded-lg shadow p-6">
          <h2 className="text-sm font-medium text-gray-500">Open Notifications</h2>
          <p className="mt-2 text-3xl font-semibold">—</p>
          <p className="mt-1 text-xs text-gray-400">
            Sprint 50: not yet wired to /v1/notifications
          </p>
        </div>
      </div>
    </div>
  );
}