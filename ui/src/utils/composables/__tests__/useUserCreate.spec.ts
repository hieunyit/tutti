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
import { useTuttiProtectionStore } from '@/stores/tuttiProtectionStore';
import { RuntimeStorageService } from '@/services/RuntimeStorageService';

vi.mock('@/services/TuttiService', () => ({
  TuttiService: { createUser: vi.fn(), getUsers: vi.fn() },
}));
vi.mock('@/services/RuntimeStorageService', () => ({
  RuntimeStorageService: { list: vi.fn() },
}));
vi.mock('@/utils/helpers/BucketDiscoveryHelper', () => ({
  BucketDiscoveryHelper: {
    discoverBuckets: vi.fn().mockResolvedValue([]),
    discoverStorageBuckets: vi.fn().mockResolvedValue([]),
  },
}));

const SECRET = 'S3cr3t-value-shown-once';

function storage(name: string, configured: boolean) {
  return {
    name,
    address: 'https://storage.example',
    provider: 'minio',
    defaultRegion: 'us-east-1',
    isSecure: true,
    httpTimeout: '5s',
    healthCheckInterval: '5s',
    connectionAccount: { configured },
  };
}

async function readyController() {
  const ctl = useUserCreate();

  await ctl.init();

  return ctl;
}

describe('useUserCreate', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    window.localStorage.clear();
    window.sessionStorage.clear();
    vi.mocked(TuttiService.createUser).mockReset();
    vi.spyOn(useTuttiProtectionStore(), 'refresh').mockResolvedValue();
    vi.mocked(TuttiService.getUsers).mockResolvedValue({
      users: [{ name: 'taken' }],
      mainStorage: 'main',
      storages: ['main'],
      dynamicCredentialsEnabled: true,
    });
    vi.mocked(RuntimeStorageService.list).mockReset();
    vi.mocked(RuntimeStorageService.list).mockResolvedValue({
      storages: [storage('main', true)],
    });
    vi.mocked(TuttiService.createUser).mockResolvedValue({
      accessKey: { accessKeyId: 'AK1', secretAccessKey: SECRET },
    });
  });

  it('canNext stays false until the name is valid', async () => {
    useTuttiUsersStore().users = [{ name: 'taken' }] as never;

    const { form, canNext } = await readyController();

    expect(canNext.value).toBe(false);
    form.value.name = '-bad';
    expect(canNext.value).toBe(false);
    form.value.name = 'taken';
    expect(canNext.value).toBe(false);
    form.value.name = 'alice';
    expect(canNext.value).toBe(true);
  });

  it('submit calls createUser once and keeps the secret out of storage', async () => {
    const { form, submit, created } = await readyController();

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

  it('submit returns the name and keeps the draft until the secret is dismissed', async () => {
    const { form, submit, dismissSecret } = await readyController();

    form.value.name = 'alice';

    const result = await submit();

    expect(result.name).toBe('alice');
    expect(form.value.name).toBe('alice');

    dismissSecret();

    expect(form.value.name).toBe('');
  });

  it('custom mode needs valid bucket patterns', async () => {
    const { form, step, canNext } = await readyController();

    step.value = 'permission';
    form.value.grantMode = 'custom';
    form.value.buckets = '';
    expect(canNext.value).toBe(false);
    form.value.buckets = 'logs-*, data';
    expect(canNext.value).toBe(true);
  });

  it('requires legacy storage credentials and sends them for an all-buckets user without persisting secrets', async () => {
    vi.mocked(RuntimeStorageService.list).mockResolvedValueOnce({
      storages: [storage('main', false), storage('replica', true)],
    });
    const ctl = await readyController();

    ctl.form.value.name = 'alice';
    expect(ctl.credentials.value.map((row) => row.storage)).toEqual(['main']);
    expect(ctl.identityValid.value).toBe(false);
    await expect(ctl.submit()).rejects.toThrow(
      'completeIdentityAndCredentials',
    );
    expect(TuttiService.createUser).not.toHaveBeenCalled();
    ctl.credentials.value[0]!.accessKeyId = 'EXISTING-S3-ACCESS';
    ctl.credentials.value[0]!.secretAccessKey = 'existing-storage-secret';
    expect(ctl.identityValid.value).toBe(true);
    await ctl.submit();
    expect(TuttiService.createUser).toHaveBeenCalledWith(
      expect.objectContaining({
        credentials: [
          {
            storage: 'main',
            accessKeyId: 'EXISTING-S3-ACCESS',
            secretAccessKey: 'existing-storage-secret',
          },
        ],
        verify: true,
        policy: expect.objectContaining({ buckets: ['*'] }),
      }),
    );
    expect(
      JSON.stringify({ ...window.localStorage, ...window.sessionStorage }),
    ).not.toContain('existing-storage-secret');
    expect(ctl.credentials.value).toEqual([]);
  });

  it('fails closed if account metadata cannot load and recovers after retry', async () => {
    vi.mocked(RuntimeStorageService.list).mockRejectedValueOnce(
      new Error('offline'),
    );
    const ctl = await readyController();

    ctl.form.value.name = 'alice';
    expect(ctl.credentialsError.value).toBe(true);
    expect(ctl.identityValid.value).toBe(false);
    await expect(ctl.submit()).rejects.toThrow(
      'completeIdentityAndCredentials',
    );
    await ctl.loadCredentialRequirements();
    expect(ctl.credentialsError.value).toBe(false);
    expect(ctl.identityValid.value).toBe(true);
  });
});
