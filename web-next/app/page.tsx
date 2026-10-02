/**
 * Sprint 50: Landing page. Redirects authenticated users to /dashboard,
 * otherwise shows a login link.
 */
import Link from 'next/link';
import { getServerSession } from '@/lib/auth-server';

export default async function Home() {
  const session = await getServerSession();
  if (session) {
    return (
      <main className="flex min-h-screen items-center justify-center">
        <div className="text-center">
          <h1 className="text-4xl font-bold mb-4">FMCG Wallet</h1>
          <p className="mb-6 text-gray-600">Welcome, {session.user_id}</p>
          <Link
            href="/dashboard"
            className="inline-block rounded-md bg-brand-600 px-4 py-2 text-white hover:bg-brand-700"
          >
            Go to dashboard
          </Link>
        </div>
      </main>
    );
  }

  return (
    <main className="flex min-h-screen items-center justify-center">
      <div className="text-center">
        <h1 className="text-4xl font-bold mb-4">FMCG Wallet</h1>
        <p className="mb-6 text-gray-600">
          Hybrid wallet backend for FMCG/F&amp;B distributors
        </p>
        <Link
          href="/login"
          className="inline-block rounded-md bg-brand-600 px-4 py-2 text-white hover:bg-brand-700"
        >
          Sign in
        </Link>
      </div>
    </main>
  );
}