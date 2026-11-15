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

import { describe, expect, it } from 'vitest';
import {
  AUDIT_ACTION_FILTERS,
  AUDIT_FAMILY_FILTERS,
  actionTone,
  familyKey,
  hasChanges,
  humanize,
  parseAction,
} from '../auditLabels';
import i18nAudit from '../i18nAudit';
import type { AuditLogEntry } from '@/utils/types/audit';

describe('auditLabels', () => {
  it('maps plural table names to the label key', () => {
    expect(familyKey('storages')).toBe('storage');
    expect(familyKey('System_Settings')).toBe('system_settings');
  });

  it('splits the intent suffix off an action', () => {
    expect(parseAction('PAUSE_INTENT')).toEqual({
      base: 'pause',
      intent: true,
    });
    expect(parseAction('RENAME')).toEqual({ base: 'rename', intent: false });
  });

  it('humanizes unknown values instead of showing the raw enum', () => {
    expect(humanize('dynamic_credential_salt')).toBe('Dynamic credential salt');
  });

  it('colors actions and keeps intent entries neutral', () => {
    expect(actionTone('DELETE')).toBe('danger');
    expect(actionTone('PAUSE')).toBe('warning');
    expect(actionTone('PAUSE_INTENT')).toBe('neutral');
  });

  it('detects a diff only when a snapshot exists', () => {
    const base = { id: 1 } as AuditLogEntry;

    expect(hasChanges(base)).toBe(false);
    expect(hasChanges({ ...base, afterJson: '{"a":1}' })).toBe(true);
  });

  it('has a label in every language for every filter option', () => {
    for (const lang of ['en', 'de', 'vi'] as const) {
      const messages = i18nAudit[lang] as Record<string, string>;

      AUDIT_FAMILY_FILTERS.forEach((family) =>
        expect(messages[`family_${familyKey(family)}`], family).toBeTruthy(),
      );
      AUDIT_ACTION_FILTERS.forEach((action) =>
        expect(messages[`action_${action}`], action).toBeTruthy(),
      );
    }
  });
});
