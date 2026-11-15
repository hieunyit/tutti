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

import { useI18n } from 'vue-i18n';
import messages from './i18nAudit';
import { familyKey, humanize, parseAction } from './auditLabels';
import { EMPTY_VALUE } from '@/components/kit';

/** Local-scope i18n shared by the audit components (follows the app language). */
export function useAuditI18n() {
  const { t, te } = useI18n({ useScope: 'local', messages });

  function familyLabel(family: string): string {
    if (!family) return EMPTY_VALUE;

    const key = `family_${familyKey(family)}`;

    return te(key) ? t(key) : humanize(family);
  }

  function actionLabel(action: string): string {
    if (!action) return EMPTY_VALUE;

    const { base, intent } = parseAction(action);
    const key = `action_${base}`;
    const label = te(key) ? t(key) : humanize(base);

    return intent ? t('action_intent', { action: label }) : label;
  }

  return { t, familyLabel, actionLabel };
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
