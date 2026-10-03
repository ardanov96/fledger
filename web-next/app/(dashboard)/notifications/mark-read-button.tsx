/**
 * Sprint 55: Mark notification as read button (client component).
 * Calls PATCH /v1/notifications/{id}/read on click.
 */
'use client';

import { useState, useTransition } from 'react';
import { useRouter } from 'next/navigation';
import { markNotificationRead } from '@/lib/api';

export function MarkReadButton({ id }: { id: string }) {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [done, setDone] = useState(false);

  const onClick = () => {
    startTransition(async () => {
      try {
        await markNotificationRead(id);
        setDone(true);
        router.refresh();
      } catch {
        // ignore — silent failure; user can retry
      }
    });
  };

  return (
    <button
      onClick={onClick}
      disabled={pending || done}
      className="rounded bg-gray-100 px-2 py-1 text-xs text-gray-600 hover:bg-gray-200 disabled:opacity-50"
    >
      {done ? '✓ read' : pending ? '…' : 'Mark read'}
    </button>
  );
}