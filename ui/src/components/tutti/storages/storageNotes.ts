/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */
import type { StatusTone } from '@/components/kit';
import type { StorageRow } from '@/utils/helpers/StoragesPageHelper';
import { StorageTestHelper } from '@/utils/helpers/StorageTestHelper';
import type { RuntimeStorage } from '@/utils/types/runtimeStorage';

export interface StorageNote {
  key: 'healthWarning' | 'noAccount' | 'testFailed';
  tone: StatusTone;
  /** i18n key of the label */
  label: string;
}

export function healthTone(status: string): StatusTone {
  if (status === 'available') return 'success';

  return status === 'unavailable' ? 'danger' : 'neutral';
}

/** True when the storage has no connection account (replication needs one). */
export function lacksAccount(storage?: RuntimeStorage): boolean {
  return !!storage && storage.connectionAccount?.configured !== true;
}

/**
 * What needs attention on a storage. A healthy, configured storage returns
 * nothing: the list shows a status only when someone has to act.
 */
export function storageNotes(
  storage: RuntimeStorage | undefined,
  row: Pick<StorageRow, 'health'>,
): StorageNote[] {
  const notes: StorageNote[] = [];
  const failed =
    StorageTestHelper.counts(storage?.lastTest).fail > 0 ||
    storage?.lastTest?.ok === false;

  if (row.health.warning) {
    notes.push({
      key: 'healthWarning',
      tone: 'warning',
      label: 'errorBadge',
    });
  }

  if (lacksAccount(storage)) {
    notes.push({ key: 'noAccount', tone: 'warning', label: 'noAccountBadge' });
  }

  if (failed) {
    notes.push({
      key: 'testFailed',
      tone: 'danger',
      label: 'testFailedBadge',
    });
  }

  return notes;
}
