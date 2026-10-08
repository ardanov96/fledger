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
        <Link href="/" className="block mb-8 font-bold text-lg">
          FMCG Wallet
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