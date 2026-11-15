/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */

/** Row model and cell text of the access keys list (table and cards share them). */
import { EMPTY_VALUE } from '@/components/ui/status';
import { formatDateTime, formatTime } from '@/utils/helpers/DateTimeHelper';
import {
  ProxyAccountHelper,
  type ProxyAccountKeyRow,
} from '@/utils/helpers/ProxyAccountHelper';
import type { TuttiProxyAccount } from '@/utils/types/tutti';

export type KeyRow = ProxyAccountKeyRow & { user: string };
export type Translate = (
  key: string,
  params?: Record<string, unknown>,
) => string;

export function toKeyRows(accounts: TuttiProxyAccount[]): KeyRow[] {
  return accounts.flatMap((account) =>
    ProxyAccountHelper.getKeyRows(account).map((row) => ({
      ...row,
      user: account.user,
    })),
  );
}

export function isDisabled(row: KeyRow): boolean {
  return !!row.dynamicKey && ProxyAccountHelper.isDisabled(row.dynamicKey);
}

/** "Config file", "Config file (storage x)" or "Managed" */
export function sourceLabel(source: string, t: Translate): string {
  if (source === 'dynamic') return t('sourceManaged');

  if (source.startsWith('storage:')) {
    return t('sourceConfigStorage', { name: source.slice('storage:'.length) });
  }

  return t('sourceConfig');
}

export function dateCell(value?: string): string {
  return ProxyAccountHelper.hasTime(value)
    ? formatDateTime(value)
    : EMPTY_VALUE;
}

/** "old secret valid until HH:MM:SS", only while it is still valid */
export function oldSecretNote(row: KeyRow, t: Translate): string | null {
  const expires = row.dynamicKey?.previousSecretExpiresAt;

  if (!ProxyAccountHelper.hasTime(expires)) return null;

  return Date.parse(expires as string) > Date.now()
    ? t('oldSecretValidUntil', { time: formatTime(expires) })
    : null;
}
