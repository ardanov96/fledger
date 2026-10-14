/**
 * Sprint 50: Dashboard layout.
 * Shared header + sidebar for authenticated pages.
 *
 * Layouts in App Router wrap all child pages in the same directory.
 * This file applies to /dashboard, /accounts, /transfers, /invoices, etc.
 */
import Link from 'next/link';

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="flex min-h-screen bg-gray-50">
      {/* Sidebar */}
      <nav className="w-56 bg-white border-r border-gray-200 px-3 py-6">
        <Link href="/" className="flex items-center gap-2.5 mb-8 font-bold text-lg text-gray-900">
          <div className="w-7 h-7 rounded-lg bg-gradient-to-br from-blue-700 to-sky-600 flex items-center justify-center shadow-sm border border-white/20">
            <svg width="16" height="16" viewBox="0 0 32 32" fill="none">
              <rect x="6" y="6" width="5.5" height="20" rx="2.75" fill="#ffffff"/>
              <rect x="14.5" y="6" width="12" height="5.5" rx="2.75" fill="#38bdf8"/>
              <rect x="14.5" y="14.5" width="8" height="5.5" rx="2.75" fill="#34d399"/>
            </svg>
          </div>
          <span>Fledger Core</span>
        </Link>
        <ul className="space-y-2 text-sm">
          <li><Link href="/dashboard" className="text-gray-700 hover:text-brand-600">Dashboard</Link></li>
          <li><Link href="/accounts" className="text-gray-700 hover:text-brand-600">Accounts</Link></li>
          <li><Link href="/transfers" className="text-gray-700 hover:text-brand-600">Transfers</Link></li>
          <li><Link href="/invoices" className="text-gray-700 hover:text-brand-600">Invoices</Link></li>
          <li><Link href="/notifications" className="text-gray-700 hover:text-brand-600">Notifications</Link></li>
        </ul>
      </nav>

      {/* Main */}
      <main className="flex-1 px-8 py-6">{children}</main>
    </div>
  );
}