/**
 * Sprint 55: Notifications feed page.
 * Migrated from web/public/notifications.html (vanilla JS).
 *
 * Sprint 28 introduced the in-app notification feed via GET /v1/notifications.
 * Sprint 55 surfaces it in the Next.js dashboard.
 */
import { listNotifications, markNotificationRead, type Notification } from '@/lib/api';
import { getServerSession } from '@/lib/auth-server';
import { MarkReadButton } from './mark-read-button';

export const dynamic = 'force-dynamic';

export default async function NotificationsPage() {
  const session = await getServerSession();
  if (!session) {
    return (
      <div className="p-8 text-center text-gray-600">
        Please <a href="/login" className="text-brand-600 underline">sign in</a> to view notifications.
      </div>
    );
  }

  let notifications: Notification[] = [];
  let error: string | null = null;
  try {
    const response = await listNotifications({ limit: 100 });
    notifications = response.data;
  } catch (err) {
    error = err instanceof Error ? err.message : 'Failed to load notifications';
  }

  const unread = notifications.filter((n) => n.status === 'unread').length;

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Notifications</h1>
        <span className="text-sm text-gray-500">
          {unread} unread / {notifications.length} total
        </span>
      </div>

      {error && (
        <div className="rounded-md bg-red-50 p-3 mb-4 text-sm text-red-700">{error}</div>
      )}

      <div className="space-y-3">
        {notifications.length === 0 ? (
          <div className="bg-white rounded-lg shadow p-8 text-center text-sm text-gray-500">
            No notifications.
          </div>
        ) : (
          notifications.map((n) => (
            <div
              key={n.id}
              className={`bg-white rounded-lg shadow p-4 ${
                n.status === 'unread' ? 'border-l-4 border-brand-500' : 'opacity-70'
              }`}
            >
              <div className="flex items-start justify-between">
                <div>
                  <div className="flex items-center gap-2">
                    <SeverityBadge severity={n.severity} />
                    <h3 className="font-medium">{n.title}</h3>
                  </div>
                  {n.body && Object.keys(n.body).length > 0 && (
                    <pre className="mt-2 text-xs text-gray-600 whitespace-pre-wrap">
                      {JSON.stringify(n.body, null, 2)}
                    </pre>
                  )}
                  <p className="mt-2 text-xs text-gray-400">
                    {new Date(n.created_at).toLocaleString()}
                  </p>
                </div>
                {n.status === 'unread' && <MarkReadButton id={n.id} />}
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  );
}

function SeverityBadge({ severity }: { severity: 'info' | 'warn' | 'critical' }) {
  const colors = {
    info: 'bg-blue-100 text-blue-800',
    warn: 'bg-yellow-100 text-yellow-800',
    critical: 'bg-red-100 text-red-800',
  };
  return (
    <span
      className={`inline-flex rounded px-1.5 py-0.5 text-[10px] font-semibold uppercase ${colors[severity]}`}
    >
      {severity}
    </span>
  );
}