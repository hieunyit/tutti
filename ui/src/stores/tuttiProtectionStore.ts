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

import axios from 'axios';
import { defineStore } from 'pinia';
import { computed, reactive, toRefs } from 'vue';
import { ChorusService } from '@/services/ChorusService';
import { ProtectionGroupService } from '@/services/ProtectionGroupService';
import { TuttiService } from '@/services/TuttiService';
import { ProtectionGroupHelper } from '@/utils/helpers/ProtectionGroupHelper';
import type {
  ProtectionGroup,
  ProtectionGroupForm,
  ProtectionGroupStatus,
  ProtectionWriters,
} from '@/utils/types/protection';

export const PROTECTION_POLL_MS = 5000;

interface ProtectionState {
  isLoading: boolean;
  isLoaded: boolean;
  hasError: boolean;
  lastUpdated: number | null;
  groups: ProtectionGroup[];
  /** by group id; a group whose status failed to load is missing */
  statuses: Record<string, ProtectionGroupStatus>;
  automaticFrozen: boolean | null;
  /** options for the dialog */
  userNames: string[];
  storageNames: string[];
  proxyIds: string[];
  optionsLoaded: boolean;
  /** detail page */
  detail: ProtectionGroupStatus | null;
  detailLoading: boolean;
  detailMissing: boolean;
  detailError: boolean;
  writers: ProtectionWriters;
  writersError: boolean;
}

function getInitialState(): ProtectionState {
  return {
    isLoading: false,
    isLoaded: false,
    hasError: false,
    lastUpdated: null,
    groups: [],
    statuses: {},
    automaticFrozen: null,
    userNames: [],
    storageNames: [],
    proxyIds: [],
    optionsLoaded: false,
    detail: null,
    detailLoading: false,
    detailMissing: false,
    detailError: false,
    writers: {},
    writersError: false,
  };
}

/** Backend message (any gRPC code) for inline display. */
export function backendMessage(error: unknown): string | null {
  if (!axios.isAxiosError(error)) return null;

  const data = error.response?.data as { message?: string } | undefined;

  return data?.message ? data.message : null;
}

export const useTuttiProtectionStore = defineStore('tuttiProtection', () => {
  const state = reactive<ProtectionState>(getInitialState());
  let timer: ReturnType<typeof setInterval> | null = null;
  let requestId = 0;

  const summary = computed(() =>
    ProtectionGroupHelper.summary(state.groups, state.statuses),
  );
  const attentionCount = computed(() =>
    ProtectionGroupHelper.attentionCount(state.groups, state.statuses),
  );

  async function refresh() {
    const id = (requestId += 1);

    state.isLoading = true;

    try {
      const [list, frozen] = await Promise.all([
        ProtectionGroupService.list(),
        ProtectionGroupService.getAutomaticFreeze().catch(() => null),
      ]);
      const groups = list.groups ?? [];
      const statuses: Record<string, ProtectionGroupStatus> = {};

      // best effort per group: a failing Get leaves that row without health
      await Promise.all(
        groups.map(async (group) => {
          try {
            statuses[group.id] = await ProtectionGroupService.get(group.id);
          } catch {
            /* status unknown */
          }
        }),
      );

      if (id !== requestId) return;

      state.groups = groups;
      state.statuses = statuses;
      state.automaticFrozen = frozen;
      state.hasError = false;
      state.isLoaded = true;
      state.lastUpdated = Date.now();
    } catch {
      if (id === requestId) state.hasError = true;
    } finally {
      if (id === requestId) state.isLoading = false;
    }
  }

  async function loadOptions() {
    const [users, storages, proxies] = await Promise.all([
      TuttiService.getUsers().catch(() => null),
      ChorusService.getStorages().catch(() => null),
      TuttiService.getProxies().catch(() => null),
    ]);

    state.userNames = (users?.users ?? []).map((u) => u.name);
    state.storageNames = (storages?.storages ?? []).map((s) => s.name);
    state.proxyIds = (proxies?.proxies ?? [])
      .map((p) => p.appId)
      .filter((x): x is string => !!x);
    state.optionsLoaded = true;
  }

  async function loadDetail(id: string, quiet = false) {
    if (!quiet) state.detailLoading = true;

    try {
      const detail = await ProtectionGroupService.get(id);

      state.detail = detail;
      state.detailMissing = false;
      state.detailError = false;
      loadWriters(id).catch(() => undefined);
    } catch (e) {
      if (axios.isAxiosError(e) && e.response?.status === 404) {
        state.detailMissing = true;
        state.detail = null;
      } else if (!state.detail) {
        state.detailError = true;
      }
    } finally {
      state.detailLoading = false;
    }
  }

  async function loadWriters(id: string) {
    try {
      state.writers = await ProtectionGroupService.listWriters(id);
      state.writersError = false;
    } catch {
      state.writersError = true;
    }
  }

  /** Each mutation returns null on success or a message (backend text if any). */
  async function run(action: () => Promise<unknown>): Promise<string | null> {
    try {
      await action();

      return null;
    } catch (e) {
      return backendMessage(e) ?? 'unknown';
    }
  }

  function createGroup(form: ProtectionGroupForm) {
    return run(async () => {
      await ProtectionGroupService.create(
        ProtectionGroupHelper.buildCreateRequest(form),
      );
      await refresh();
    });
  }

  function updateGroup(current: ProtectionGroup, form: ProtectionGroupForm) {
    return run(async () => {
      await ProtectionGroupService.update(
        ProtectionGroupHelper.buildUpdateRequest(current, form),
      );
      await Promise.all([refresh(), loadDetail(current.id, true)]);
    });
  }

  function deleteGroup(id: string) {
    return run(async () => {
      await ProtectionGroupService.remove(id);
      await refresh();
    });
  }

  function switchover(id: string, target: string) {
    return run(async () => {
      await ProtectionGroupService.switchover(id, target);
      await loadDetail(id, true);
    });
  }

  function abortSwitch(id: string) {
    return run(async () => {
      await ProtectionGroupService.abortSwitch(id);
      await loadDetail(id, true);
    });
  }

  function setAutomaticFreeze(frozen: boolean) {
    return run(async () => {
      await ProtectionGroupService.setAutomaticFreeze(frozen);
      state.automaticFrozen = frozen;
    });
  }

  function clearWriter(id: string, token: string, worker: boolean) {
    return run(async () => {
      await ProtectionGroupService.clearWriter({
        id,
        token,
        worker,
        externallyFenced: true,
      });
      await loadWriters(id);
    });
  }

  function startPolling(tick: () => Promise<unknown> = refresh) {
    stopPolling();
    tick().catch(() => undefined);
    timer = setInterval(() => {
      if (typeof document === 'undefined' || !document.hidden) {
        tick().catch(() => undefined);
      }
    }, PROTECTION_POLL_MS);
  }

  function stopPolling() {
    requestId += 1;

    if (timer !== null) clearInterval(timer);

    timer = null;
  }

  function resetDetail() {
    state.detail = null;
    state.detailMissing = false;
    state.detailError = false;
    state.writers = {};
    state.writersError = false;
  }

  function $reset() {
    stopPolling();
    Object.assign(state, getInitialState());
  }

  return {
    ...toRefs(state),
    summary,
    attentionCount,
    refresh,
    loadOptions,
    loadDetail,
    loadWriters,
    createGroup,
    updateGroup,
    deleteGroup,
    switchover,
    abortSwitch,
    setAutomaticFreeze,
    clearWriter,
    startPolling,
    stopPolling,
    resetDetail,
    $reset,
  };
});
