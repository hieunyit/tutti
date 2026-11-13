/*
 * Copyright © 2026 hieunyit
 *
 *  Licensed under the GNU Affero General Public License, Version 3.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  https://www.gnu.org/licenses/agpl-3.0.html
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

import { watchEffect } from 'vue';
import { useI18n } from 'vue-i18n';
import messages from './i18nAudit';
import { useAuditLogStore } from '@/stores/tuttiAuditLogStore';
import { EMPTY_VALUE } from '@/components/kit';
import type { StatusTone } from '@/components/kit';

/**
 * Local-scope i18n shared by the audit components. The page's vi/en toggle
 * lives in the store (`localeOverride`) so every component switches together;
 * without an override the app language applies.
 */
export function useAuditI18n() {
  const store = useAuditLogStore();
  const { t, locale } = useI18n({ useScope: 'local', messages });

  watchEffect(() => {
    if (store.localeOverride)
      locale.value = store.localeOverride as 'en' | 'vi';
  });

  function familyLabel(family: string): string {
    if (!family) return EMPTY_VALUE;

    const key = `family.${family}`;
    const translated = t(key);

    return translated !== key ? translated : family;
  }

  function actionLabel(action: string): string {
    if (!action) return EMPTY_VALUE;

    const key = `action.${action.toLowerCase()}`;
    const translated = t(key);

    return translated !== key ? translated : action;
  }

  return { t, locale, familyLabel, actionLabel };
}

export function actionTone(action: string): StatusTone {
  switch ((action || '').trim().toLowerCase()) {
    case 'create':

    case 'unblock':
      return 'success';

    case 'update':

    case 'set':

    case 'generation':
      return 'info';

    case 'delete':

    case 'block':
      return 'danger';

    case 'rotate':

    case 'archive':
      return 'warning';

    default:
      return 'neutral';
  }
}

export function formatJsonPayload(raw?: string): string {
  if (!raw || !raw.trim()) return '';

  try {
    return JSON.stringify(JSON.parse(raw), null, 2);
  } catch {
    return raw;
  }
}

function parseObject(raw?: string): Record<string, unknown> | null {
  if (!raw || !raw.trim()) return null;

  try {
    const parsed = JSON.parse(raw);

    return parsed && typeof parsed === 'object' && !Array.isArray(parsed)
      ? (parsed as Record<string, unknown>)
      : null;
  } catch {
    return null;
  }
}

export function changedTopLevelKeys(before?: string, after?: string): string[] {
  const a = parseObject(before);
  const b = parseObject(after);

  if (!a && !b) return [];

  const keys = new Set([...Object.keys(a ?? {}), ...Object.keys(b ?? {})]);

  return [...keys].filter(
    (key) => JSON.stringify(a?.[key]) !== JSON.stringify(b?.[key]),
  );
}
