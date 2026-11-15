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
import { mount } from '@vue/test-utils';
import ReplicationsShortList from '../ReplicationsShortList.vue';
import { tolerateNaiveStyles } from '@/components/kit/__tests__/kitTestUtils';

tolerateNaiveStyles();

vi.mock('@clyso/clyso-ui-kit', () => ({
  CShortList: {
    props: ['list'],
    template:
      '<ul><li v-for="item in list" :key="item.idStr"><slot :item="item" /></li></ul>',
  },
}));

function render(id: Record<string, string>, name?: string) {
  return mount(ReplicationsShortList, {
    props: { replications: [{ id, idStr: 'x', name } as never] },
  }).text();
}

describe('ReplicationsShortList', () => {
  it('renders the auto label for an unnamed storage-level replication', () => {
    expect(
      render({ user: '', fromStorage: 'A', toStorage: 'B' }).replace(
        /\s+/g,
        ' ',
      ),
    ).toBe('A → B · all buckets');
  });

  it('keeps showing a real user', () => {
    expect(
      render({ user: 'alice', fromStorage: 'A', toStorage: 'B' }),
    ).toContain('alice:');
  });

  it('shows the replication name when it has one', () => {
    const text = render(
      {
        user: '',
        fromStorage: 'A',
        toStorage: 'B',
        fromBucket: 'b',
        toBucket: 'b',
      },
      'nightly copy',
    );

    expect(text).toContain('nightly copy');
    expect(text).not.toContain('→');
  });

  it('falls back to from → to · bucket without a name', () => {
    expect(
      render({
        user: '',
        fromStorage: 'A',
        toStorage: 'B',
        fromBucket: 'b',
        toBucket: 'b',
      }),
    ).toContain('A → B · b');
  });
});
