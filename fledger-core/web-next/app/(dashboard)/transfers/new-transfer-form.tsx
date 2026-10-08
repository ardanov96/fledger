/**
 * Sprint 54: Client component for new transfer form.
 * Submits to POST /v1/transfers (handler in cmd/api).
 */
'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';

interface Account {
  id: string;
  code: string;
  name: string;
  currency: string;
}

export function NewTransferForm({ tenantId }: { tenantId: string }) {
  const router = useRouter();
  const [fromAccountId, setFromAccountId] = useState('');
  const [toAccountId, setToAccountId] = useState('');
  const [amount, setAmount] = useState(''); // human-entered, converted below
  const [description, setDescription] = useState('');
  const [idempotencyKey] = useState(() => crypto.randomUUID());
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  // Sprint 54: real impl fetches accounts via /v1/accounts.
  // For Sprint 54 PoC, we accept raw account UUIDs.
  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      const amountMinor = Math.round(parseFloat(amount) * 100);
      if (!Number.isFinite(amountMinor) || amountMinor <= 0) {
        throw new Error('amount must be positive');
      }
      const res = await fetch('/v1/transfers', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          from_account_id: fromAccountId,
          to_account_id: toAccountId,
          amount: amountMinor,
          currency: 'IDR',
          idempotency_key: idempotencyKey,
          description,
        }),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({ message: res.statusText }));
        throw new Error(body.message || body.code || `HTTP ${res.status}`);
      }
      router.refresh();
      setAmount('');
      setDescription('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <form onSubmit={onSubmit} className="space-y-3 pt-3">
      {error && <div className="rounded bg-red-50 p-2 text-sm text-red-700">{error}</div>}
      <div className="grid grid-cols-2 gap-3">
        <div>
          <label className="block text-xs font-medium text-gray-500 mb-1">From account ID</label>
          <input
            value={fromAccountId}
            onChange={(e) => setFromAccountId(e.target.value)}
            required
            className="w-full rounded border border-gray-300 px-2 py-1 text-sm"
            placeholder="uuid"
          />
        </div>
        <div>
          <label className="block text-xs font-medium text-gray-500 mb-1">To account ID</label>
          <input
            value={toAccountId}
            onChange={(e) => setToAccountId(e.target.value)}
            required
            className="w-full rounded border border-gray-300 px-2 py-1 text-sm"
            placeholder="uuid"
          />
        </div>
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div>
          <label className="block text-xs font-medium text-gray-500 mb-1">Amount (IDR)</label>
          <input
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
            required
            type="number"
            step="0.01"
            className="w-full rounded border border-gray-300 px-2 py-1 text-sm"
            placeholder="0.00"
          />
        </div>
        <div>
          <label className="block text-xs font-medium text-gray-500 mb-1">Description</label>
          <input
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            className="w-full rounded border border-gray-300 px-2 py-1 text-sm"
            placeholder="optional"
          />
        </div>
      </div>
      <button
        type="submit"
        disabled={submitting}
        className="rounded bg-brand-600 px-4 py-1.5 text-sm text-white hover:bg-brand-700 disabled:opacity-50"
      >
        {submitting ? 'Posting…' : 'Post transfer'}
      </button>
    </form>
  );
}