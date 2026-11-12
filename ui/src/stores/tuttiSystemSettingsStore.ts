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

import { defineStore } from 'pinia';
import { reactive, toRefs, computed } from 'vue';
import axios from 'axios';
import { TuttiService } from '@/services/TuttiService';
import {
  toProtoDuration,
  validateSystemSettings,
  WORKER_DURATION_FIELDS,
} from '@/utils/helpers/SystemSettingsHelper';
import type {
  TuttiSystemSettings,
  TuttiUpdateSystemSettingsRequest,
} from '@/utils/types/tutti';

interface SystemSettingsState {
  isLoading: boolean;
  isSaving: boolean;
  hasError: boolean;
  errorMessage: string;
  conflictError: boolean;
  saveSuccess: boolean;
  settings: TuttiSystemSettings | null;
  draft: TuttiUpdateSystemSettingsRequest;
}

function getDefaultDraft(): TuttiUpdateSystemSettingsRequest {
  return {
    features: {
      tagging: false,
      acl: false,
      lifecycle: false,
      policy: false,
      preserveAclGrants: false,
      versioning: false,
    },
    worker: {
      concurrency: 10,
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
      enabled: false,
      baseUrl: '',
    },
  };
}

function cloneSettingsToDraft(
  s: TuttiSystemSettings | null,
): TuttiUpdateSystemSettingsRequest {
  if (!s) return getDefaultDraft();

  return {
    expectedVersion: s.version,
    features: {
      tagging: s.features?.tagging ?? false,
      acl: s.features?.acl ?? false,
      lifecycle: s.features?.lifecycle ?? false,
      policy: s.features?.policy ?? false,
      preserveAclGrants: s.features?.preserveAclGrants ?? false,
      versioning: s.features?.versioning ?? false,
    },
    worker: {
      concurrency: s.worker?.concurrency ?? 10,
      shutdownTimeout: s.worker?.shutdownTimeout ?? '5s',
      queueUpdateInterval: s.worker?.queueUpdateInterval ?? '3s',
      taskCheckInterval: s.worker?.taskCheckInterval ?? '1s',
      delayedTaskCheckInterval: s.worker?.delayedTaskCheckInterval ?? '3s',
      swiftRetryInterval: s.worker?.swiftRetryInterval ?? '1m',
      pauseRetryInterval: s.worker?.pauseRetryInterval ?? '30s',
      switchRetryInterval: s.worker?.switchRetryInterval ?? '30s',
      backlogHighWatermark: s.worker?.backlogHighWatermark ?? 10000,
      backlogLowWatermark: s.worker?.backlogLowWatermark ?? 5000,
      bandwidthLimitBytes: s.worker?.bandwidthLimitBytes ?? '2097152',
      bufferBudgetBytes: s.worker?.bufferBudgetBytes ?? '268435456',
    },
    lock: {
      overlap: s.lock?.overlap ?? '2s',
    },
    webhook: {
      enabled: s.webhook?.enabled ?? false,
      baseUrl: s.webhook?.baseUrl ?? '',
    },
  };
}

export const useTuttiSystemSettingsStore = defineStore(
  'tuttiSystemSettings',
  () => {
    const state = reactive<SystemSettingsState>({
      isLoading: false,
      isSaving: false,
      hasError: false,
      errorMessage: '',
      conflictError: false,
      saveSuccess: false,
      settings: null,
      draft: getDefaultDraft(),
    });

    const hasChanges = computed(() => {
      if (!state.settings) return false;

      const original = cloneSettingsToDraft(state.settings);

      return JSON.stringify(original) !== JSON.stringify(state.draft);
    });

    const validationErrors = computed(() =>
      validateSystemSettings(state.draft),
    );
    const canSave = computed(
      () =>
        Boolean(state.settings) &&
        hasChanges.value &&
        !state.isLoading &&
        !state.isSaving &&
        Object.keys(validationErrors.value).length === 0,
    );
    let loadSequence = 0;

    async function load() {
      if (state.isSaving) return;

      const sequence = ++loadSequence;

      state.isLoading = true;

      state.hasError = false;
      state.errorMessage = '';
      state.conflictError = false;
      state.saveSuccess = false;

      try {
        const res = await TuttiService.getSystemSettings();

        if (sequence !== loadSequence) return;

        state.settings = res;
        state.draft = cloneSettingsToDraft(res);
      } catch (e) {
        if (sequence !== loadSequence) return;

        state.hasError = true;
        state.errorMessage = axios.isAxiosError(e)
          ? e.response?.data?.message || e.message
          : e instanceof Error
            ? e.message
            : String(e);
      } finally {
        if (sequence === loadSequence) state.isLoading = false;
      }
    }

    async function save(): Promise<boolean> {
      if (!canSave.value) return false;

      state.isSaving = true;
      state.hasError = false;
      state.errorMessage = '';
      state.conflictError = false;
      state.saveSuccess = false;

      try {
        const snapshot = JSON.stringify(state.draft);
        const payload = cloneSettingsToDraft({
          ...state.draft,
          version: state.settings!.version,
        });

        for (const key of WORKER_DURATION_FIELDS) {
          payload.worker![key] = toProtoDuration(payload.worker![key]!)!;
        }

        payload.lock!.overlap = toProtoDuration(payload.lock!.overlap!)!;
        payload.webhook!.baseUrl = payload.webhook!.baseUrl?.trim();
        const res = await TuttiService.updateSystemSettings(payload);

        state.settings = res;

        // Keep edits made programmatically while the request was in flight.
        if (JSON.stringify(state.draft) === snapshot) {
          state.draft = cloneSettingsToDraft(res);
        } else {
          state.draft.expectedVersion = res.version;
        }

        state.saveSuccess = true;

        return true;
      } catch (e) {
        if (axios.isAxiosError(e)) {
          const status = e.response?.status;
          const code = (
            e.response?.data as { code?: number; message?: string } | undefined
          )?.code;
          const msg = (e.response?.data as { message?: string } | undefined)
            ?.message;

          if (status === 409 || status === 412 || code === 9 || code === 10) {
            state.conflictError = true;
          }

          state.errorMessage = msg || e.message;
        } else {
          state.errorMessage = e instanceof Error ? e.message : String(e);
        }

        state.hasError = true;

        return false;
      } finally {
        state.isSaving = false;
      }
    }

    function resetDraft() {
      state.draft = cloneSettingsToDraft(state.settings);
      state.saveSuccess = false;
      state.conflictError = false;
      state.hasError = false;
    }

    return {
      ...toRefs(state),
      hasChanges,
      validationErrors,
      canSave,
      load,
      save,
      resetDraft,
    };
  },
);
