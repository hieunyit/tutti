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
import { describe, expect, it } from 'vitest';
import { createI18n } from 'vue-i18n';
import { createMemoryHistory, createRouter } from 'vue-router';
import DocsGettingStarted from '../DocsGettingStarted.vue';
import { DOCS } from '@/utils/helpers/DocsHelper';
import { RouteName } from '@/utils/types/router';

function mountIt() {
  const stub = { template: '<div />' };
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      RouteName.TUTTI_STORAGES,
      RouteName.TUTTI_ADD_REPLICATION,
      RouteName.TUTTI_REPLICATION,
      RouteName.TUTTI_API_REFERENCE,
    ].map((name) => ({ path: `/${name}`, name, component: stub })),
  });

  return mount(DocsGettingStarted, {
    global: {
      plugins: [
        router,
        createI18n({ legacy: false, locale: 'en', messages: {} }),
      ],
    },
  });
}

describe('DocsGettingStarted', () => {
  it('lists the three first steps in order with in-app links', () => {
    const wrapper = mountIt();
    const steps = wrapper.findAll('ol > li');

    expect(steps.map((s) => s.get('h3').text())).toEqual([
      'Add two storages',
      'Create a replication',
      'Watch progress',
    ]);
    expect(steps.every((s) => s.find('a').exists())).toBe(true);
  });

  it('links the long guides to the repository instead of inlining them', () => {
    const wrapper = mountIt();
    const external = wrapper.findAll('a[target="_blank"]');

    expect(external).toHaveLength(DOCS.length);
    expect(external[0]?.attributes('href')).toContain(
      'github.com/hieunyit/tutti/blob/main/docs/tutti/deployment-guide.md',
    );
    expect(wrapper.text().length).toBeLessThan(1500);
  });
});
