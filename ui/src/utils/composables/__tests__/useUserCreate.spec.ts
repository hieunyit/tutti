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

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';
import { useUserCreate } from '../useUserCreate';
import { TuttiService } from '@/services/TuttiService';
import { useTuttiUsersStore } from '@/stores/tuttiUsersStore';

vi.mock('@/services/TuttiService', () => ({
  TuttiService: { createUser: vi.fn() },
}));

const SECRET = 'S3cr3t-value-shown-once';

describe('useUserCreate', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    window.localStorage.clear();
    window.sessionStorage.clear();
    vi.mocked(TuttiService.createUser).mockReset();
    vi.mocked(TuttiService.createUser).mockResolvedValue({
      accessKey: { accessKeyId: 'AK1', secretAccessKey: SECRET },
    });
  });

  it('canNext stays false until the name is valid', () => {
    useTuttiUsersStore().users = [{ name: 'taken' }] as never;

    const { form, canNext } = useUserCreate();

    expect(canNext.value).toBe(false);
    form.value.name = '-bad';
    expect(canNext.value).toBe(false);
    form.value.name = 'taken';
    expect(canNext.value).toBe(false);
    form.value.name = 'alice';
    expect(canNext.value).toBe(true);
  });

  it('submit calls createUser once and keeps the secret out of storage', async () => {
    const { form, submit, created } = useUserCreate();

    form.value.name = 'alice';

    const result = await submit();

    expect(TuttiService.createUser).toHaveBeenCalledTimes(1);
    expect(TuttiService.createUser).toHaveBeenCalledWith(
      expect.objectContaining({
        name: 'alice',
        policy: expect.objectContaining({ buckets: ['*'] }),
      }),
    );
    expect(result.secret).toBe(SECRET);
    expect(created.value?.secretAccessKey).toBe(SECRET);

    const dump =
      JSON.stringify({ ...window.localStorage }) +
      JSON.stringify({ ...window.sessionStorage });

    expect(dump).not.toContain(SECRET);
    expect(window.location.href).not.toContain(SECRET);
  });

  it('custom mode needs valid bucket patterns', () => {
    const { form, step, canNext } = useUserCreate();

    step.value = 'permission';
    form.value.grantMode = 'custom';
    form.value.buckets = '';
    expect(canNext.value).toBe(false);
    form.value.buckets = 'logs-*, data';
    expect(canNext.value).toBe(true);
  });
});
