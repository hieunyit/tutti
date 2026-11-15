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

import { mount } from '@vue/test-utils';
import { describe, expect, it, vi } from 'vitest';
import { createI18n } from 'vue-i18n';
import DashboardKpis from '../DashboardKpis.vue';
import { tolerateNaiveStyles } from '@/components/kit/__tests__/kitTestUtils';

tolerateNaiveStyles();

vi.mock('@clyso/clyso-ui-kit', () => ({
  I18nLocale: { EN: 'en', DE: 'de', VI: 'vi' },
  I18N_DEFAULT_LOCALE: 'en',
}));

vi.mock('@/components/tutti/monitoring/useNumberFormat', () => ({
  useNumberFormat: () => (v: number) => String(v),
}));

function mountKpis(total: number) {
  return mount(DashboardKpis, {
    props: {
      storages: [
        { name: 'a', provider: 'S3', state: 'healthy' },
        { name: 'b', provider: 'S3', state: 'healthy' },
      ] as never,
      replications: {
        total,
        active: total,
        paused: 0,
        totalLag: 0,
      } as never,
      traffic: null,
    },
    global: {
      stubs: { RouterLink: { template: '<a><slot /></a>' } },
      plugins: [createI18n({ legacy: false, locale: 'en', messages: {} })],
    },
  });
}

describe('DashboardKpis', () => {
  it('shows at most 4 tiles and hides replication tiles without replications', () => {
    const empty = mountKpis(0);

    expect(empty.text()).toContain('Storages healthy');
    expect(empty.text()).toContain('Error rate');
    expect(empty.text()).not.toContain('Events behind');
    expect(empty.text()).not.toContain('Proxies reporting');
    expect(empty.text()).not.toContain('Requests');

    const full = mountKpis(3);

    expect(full.text()).toContain('Events behind');
    expect(full.findAll('a').length).toBeLessThanOrEqual(4);
  });

  it('does not repeat the label as a subtext', () => {
    expect(mountKpis(2).text()).not.toContain('Everything in sync');
  });
});
