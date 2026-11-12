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

import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useTuttiSystemSettingsStore } from '../tuttiSystemSettingsStore';
import { TuttiService } from '@/services/TuttiService';
import type { TuttiSystemSettings } from '@/utils/types/tutti';

vi.mock('@/services/TuttiService', () => ({
  TuttiService: {
    getSystemSettings: vi.fn(),
    updateSystemSettings: vi.fn(),
  },
}));

describe('useTuttiSystemSettingsStore', () => {
  const mockSettings: TuttiSystemSettings = {
    version: 1,
    updatedAt: '2026-11-11T20:00:00Z',
    features: {
      tagging: true,
      acl: false,
      lifecycle: true,
      policy: false,
      preserveAclGrants: false,
      versioning: false,
    },
    worker: {
      concurrency: 16,
      shutdownTimeout: '5s',
      queueUpdateInterval: '3s',
      taskCheckInterval: '1s',
      delayedTaskCheckInterval: '3s',
      swiftRetryInterval: '1m',
      pauseRetryInterval: '30s',
      switchRetryInterval: '30s',
      backlogHighWatermark: 10000,
      backlogLowWatermark: 5000,
      bandwidthLimitBytes: '2097152',
      bufferBudgetBytes: '268435456',
    },
    lock: {
      overlap: '2s',
    },
    webhook: {
      enabled: true,
      baseUrl: 'http://webhook.internal',
    },
  };

  beforeEach(() => {
    setActivePinia(createPinia());
    vi.clearAllMocks();
  });

  it('loads settings and populates draft', async () => {
    vi.mocked(TuttiService.getSystemSettings).mockResolvedValueOnce(
      mockSettings,
    );
    const store = useTuttiSystemSettingsStore();

    expect(store.settings).toBeNull();
    await store.load();

    expect(store.settings).toEqual(mockSettings);
    expect(store.draft.features?.tagging).toBe(true);
    expect(store.draft.worker?.concurrency).toBe(16);
    expect(store.hasChanges).toBe(false);
  });

  it('detects changes when draft is modified and resets correctly', async () => {
    vi.mocked(TuttiService.getSystemSettings).mockResolvedValueOnce(
      mockSettings,
    );
    const store = useTuttiSystemSettingsStore();

    await store.load();

    expect(store.hasChanges).toBe(false);

    store.draft.features!.acl = true;
    expect(store.hasChanges).toBe(true);

    store.resetDraft();
    expect(store.draft.features?.acl).toBe(false);
    expect(store.hasChanges).toBe(false);
  });

  it('saves updated settings successfully', async () => {
    vi.mocked(TuttiService.getSystemSettings).mockResolvedValueOnce(
      mockSettings,
    );
    const store = useTuttiSystemSettingsStore();

    await store.load();

    store.draft.worker!.concurrency = 32;

    const updatedSettings: TuttiSystemSettings = {
      ...mockSettings,
      version: 2,
      worker: { ...mockSettings.worker, concurrency: 32 },
    };

    vi.mocked(TuttiService.updateSystemSettings).mockResolvedValueOnce(
      updatedSettings,
    );

    const ok = await store.save();

    expect(ok).toBe(true);
    expect(TuttiService.updateSystemSettings).toHaveBeenCalledWith({
      ...store.draft,
      expectedVersion: 1,
      worker: { ...store.draft.worker, swiftRetryInterval: '60s' },
    });
    expect(store.settings?.version).toBe(2);
    expect(store.settings?.worker?.concurrency).toBe(32);
    expect(store.saveSuccess).toBe(true);
    expect(store.hasChanges).toBe(false);
  });

  it('handles permission denied (403 Forbidden) gracefully', async () => {
    vi.mocked(TuttiService.getSystemSettings).mockResolvedValueOnce(
      mockSettings,
    );
    const store = useTuttiSystemSettingsStore();

    await store.load();

    store.draft.worker!.concurrency = 24;
    vi.mocked(TuttiService.updateSystemSettings).mockRejectedValueOnce({
      isAxiosError: true,
      response: {
        status: 403,
        data: { message: 'permission denied: admin role required' },
      },
    });

    const ok = await store.save();

    expect(ok).toBe(false);
    expect(store.saveSuccess).toBe(false);
    expect(store.hasError).toBe(true);
    expect(store.errorMessage).toBe('permission denied: admin role required');
  });

  it('recovers from transient network 503 error on subsequent retry', async () => {
    vi.mocked(TuttiService.getSystemSettings).mockResolvedValueOnce(
      mockSettings,
    );
    const store = useTuttiSystemSettingsStore();

    await store.load();

    store.draft.worker!.concurrency = 20;

    // First attempt fails with 503
    vi.mocked(TuttiService.updateSystemSettings).mockRejectedValueOnce({
      isAxiosError: true,
      response: {
        status: 503,
        data: { message: 'service temporarily unavailable' },
      },
    });

    let ok = await store.save();

    expect(ok).toBe(false);
    expect(store.hasError).toBe(true);
    expect(store.errorMessage).toBe('service temporarily unavailable');

    // Retry succeeds
    const updatedSettings: TuttiSystemSettings = {
      ...mockSettings,
      version: 2,
      worker: { ...mockSettings.worker, concurrency: 20 },
    };

    vi.mocked(TuttiService.updateSystemSettings).mockResolvedValueOnce(
      updatedSettings,
    );

    ok = await store.save();
    expect(ok).toBe(true);
    expect(store.hasError).toBe(false);
    expect(store.errorMessage).toBe('');
    expect(store.settings?.worker?.concurrency).toBe(20);
  });

  it('does not allow saving defaults after an initial load failure', async () => {
    vi.mocked(TuttiService.getSystemSettings).mockRejectedValueOnce(
      new Error('offline'),
    );
    const store = useTuttiSystemSettingsStore();

    await store.load();
    store.draft.worker!.concurrency = 32;

    expect(store.hasError).toBe(true);
    expect(await store.save()).toBe(false);
    expect(TuttiService.updateSystemSettings).not.toHaveBeenCalled();
  });

  it('rejects invalid watermarks and blank numeric fields before calling the API', async () => {
    vi.mocked(TuttiService.getSystemSettings).mockResolvedValueOnce(
      mockSettings,
    );
    const store = useTuttiSystemSettingsStore();

    await store.load();
    store.draft.worker!.backlogLowWatermark = 12000;
    expect(store.validationErrors.backlogLowWatermark).toBe(
      'invalidWatermarkOrder',
    );
    expect(await store.save()).toBe(false);
    store.draft.worker!.backlogLowWatermark = 5000;
    store.draft.worker!.bandwidthLimitBytes = '';
    expect(await store.save()).toBe(false);
    expect(TuttiService.updateSystemSettings).not.toHaveBeenCalled();
  });

  it('keeps the latest response when configuration loads overlap', async () => {
    let resolveOld!: (value: TuttiSystemSettings) => void;

    vi.mocked(TuttiService.getSystemSettings)
      .mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            resolveOld = resolve;
          }),
      )
      .mockResolvedValueOnce({ ...mockSettings, version: 9 });
    const store = useTuttiSystemSettingsStore();
    const old = store.load();

    await store.load();
    resolveOld(mockSettings);
    await old;
    expect(store.settings?.version).toBe(9);
    expect(store.isLoading).toBe(false);
  });

  it('keeps the draft after a refresh failure and after a version conflict', async () => {
    vi.mocked(TuttiService.getSystemSettings).mockResolvedValueOnce(
      mockSettings,
    );
    const store = useTuttiSystemSettingsStore();

    await store.load();
    store.draft.worker!.concurrency = 32;
    vi.mocked(TuttiService.getSystemSettings).mockRejectedValueOnce(
      new Error('offline'),
    );
    await store.load();
    expect(store.draft.worker!.concurrency).toBe(32);
    vi.mocked(TuttiService.updateSystemSettings).mockRejectedValueOnce({
      isAxiosError: true,
      response: {
        status: 409,
        data: { code: 10, message: 'version conflict' },
      },
    });
    expect(await store.save()).toBe(false);
    expect(store.conflictError).toBe(true);
    expect(store.draft.worker!.concurrency).toBe(32);
  });

  it('blocks duplicate saves and refreshes while preserving edits made during save', async () => {
    vi.mocked(TuttiService.getSystemSettings).mockResolvedValueOnce(
      mockSettings,
    );
    const store = useTuttiSystemSettingsStore();
    let finish!: (value: TuttiSystemSettings) => void;

    await store.load();
    store.draft.worker!.concurrency = 32;
    vi.mocked(TuttiService.updateSystemSettings).mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          finish = resolve;
        }),
    );
    const saving = store.save();

    expect(await store.save()).toBe(false);
    await store.load();
    expect(TuttiService.getSystemSettings).toHaveBeenCalledTimes(1);
    store.draft.worker!.concurrency = 48;
    expect(
      vi.mocked(TuttiService.updateSystemSettings).mock.calls[0]![0].worker!
        .concurrency,
    ).toBe(32);
    finish({
      ...mockSettings,
      version: 2,
      worker: { ...mockSettings.worker, concurrency: 32 },
    });
    await saving;
    expect(store.draft.worker!.concurrency).toBe(48);
    expect(store.draft.expectedVersion).toBe(2);
    expect(store.hasChanges).toBe(true);
  });
});
