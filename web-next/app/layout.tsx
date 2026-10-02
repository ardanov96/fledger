/**
 * Sprint 50: Next.js 15 root layout.
 * Wraps every page with AuthProvider + global styles.
 *
 * Migrated from web/public/index.html (vanilla JS) + web/server.js (Node proxy).
 * The Next.js App Router handles client-side routing + layouts.
 */
import type { Metadata } from 'next';
import { AuthProvider } from '@/lib/auth';
import './globals.css';

export const metadata: Metadata = {
  title: 'FMCG Wallet',
  description: 'Hybrid wallet backend for FMCG/F&B distributors',
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body className="bg-gray-50 text-gray-900 antialiased">
        <AuthProvider>{children}</AuthProvider>
      </body>
    </html>
  );
}