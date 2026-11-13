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

import { effectScope, nextTick, ref } from 'vue';
import { afterEach, describe, expect, it } from 'vitest';
import { useStoreDraft } from '../useStoreDraft';
import { clearWizardDraft } from '@/components/kit';

const KEY = 'test-store-draft';

interface Draft {
  from: string | null;
  buckets: string[];
}

/** a stand-in for a pinia store form */
function mountForm(defaults: Draft, ready = true) {
  const from = ref(defaults.from);
  const buckets = ref(defaults.buckets);
  const isReady = ref(ready);
  const scope = effectScope();

  scope.run(() =>
    useStoreDraft<Draft>({
      draftKey: KEY,
      initial: () => ({ from: null, buckets: [] }),
      snapshot: () => ({ from: from.value, buckets: [...buckets.value] }),
      apply: (draft) => {
        if (draft.from) from.value = draft.from;

        buckets.value = draft.buckets;
      },
      ready: isReady,
    }),
  );

  return { from, buckets, isReady, scope };
}

describe('useStoreDraft', () => {
  afterEach(() => {
    clearWizardDraft(KEY);
    window.sessionStorage.clear();
  });

  it('restores step 1 values after the page is mounted again', async () => {
    const first = mountForm({ from: 's3-a', buckets: [] });

    first.from.value = 's3-b';
    first.buckets.value = ['logs', 'media'];
    await nextTick();
    first.scope.stop();

    const second = mountForm({ from: 's3-a', buckets: [] });

    expect(second.from.value).toBe('s3-b');
    expect(second.buckets.value).toEqual(['logs', 'media']);
    second.scope.stop();
  });

  it('waits for the store data before applying the draft', async () => {
    const first = mountForm({ from: 's3-a', buckets: [] });

    first.from.value = 's3-b';
    await nextTick();
    first.scope.stop();

    const second = mountForm({ from: null, buckets: [] }, false);

    expect(second.from.value).toBeNull();

    second.isReady.value = true;
    await nextTick();

    expect(second.from.value).toBe('s3-b');
    second.scope.stop();
  });

  it('does not restore anything after the draft was cleared', async () => {
    const first = mountForm({ from: 's3-a', buckets: [] });

    first.from.value = 's3-b';
    await nextTick();
    clearWizardDraft(KEY);
    first.scope.stop();

    const second = mountForm({ from: 's3-a', buckets: [] });

    expect(second.from.value).toBe('s3-a');
    second.scope.stop();
  });
});
