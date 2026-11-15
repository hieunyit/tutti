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

import { describe, expect, it, vi } from 'vitest';
import type { RouteLocationNormalized } from 'vue-router';
import metaHook from '../meta-hook';
import { i18n } from '@/i18n';
import { RouteName } from '@/utils/types/router';

// the real kit pulls in a stylesheet that node cannot import
vi.mock('@clyso/clyso-ui-kit', () => ({
  I18nLocale: { EN: 'en', DE: 'de' },
  I18N_DEFAULT_LOCALE: 'en',
}));

const run = (name: RouteName) =>
  (
    metaHook as unknown as (
      to: RouteLocationNormalized,
      from: RouteLocationNormalized,
      failure: undefined,
    ) => void
  )(
    { name } as RouteLocationNormalized,
    {} as RouteLocationNormalized,
    undefined,
  );

describe('document title per route', () => {
  it.each([
    [RouteName.TUTTI_DASHBOARD, 'Tutti | Dashboard'],
    [RouteName.TUTTI_ROUTING_RULES, 'Tutti | Routing rules'],
    [RouteName.TUTTI_USERS, 'Tutti | Users'],
    [RouteName.TUTTI_ACCESS_KEYS, 'Tutti | Access keys'],
    [RouteName.TUTTI_AUDIT_LOG, 'Tutti | Audit log'],
  ])('%s -> %s', (name, title) => {
    run(name);
    expect(document.title).toBe(title);
  });

  it('follows the app language', () => {
    (i18n.global.locale as unknown as { value: string }).value = 'de';
    run(RouteName.TUTTI_ACCESS_KEYS);
    expect(document.title).toBe('Tutti | Zugriffsschlüssel');
    (i18n.global.locale as unknown as { value: string }).value = 'en';
  });
});
