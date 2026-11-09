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
import { computed, reactive, toRefs } from 'vue';
import { ChorusService } from '@/services/ChorusService';
import { TuttiService } from '@/services/TuttiService';
import { RuntimeStorageService } from '@/services/RuntimeStorageService';
import { StorageProvider } from '@/utils/types/chorus';
import type { RuntimeStorage } from '@/utils/types/runtimeStorage';
import {
  StoragesPageHelper,
  type StoragesSources,
} from '@/utils/helpers/StoragesPageHelper';
import type {
  BucketRoutingPolicy,
  ChorusReplication,
  ChorusStorage,
  UserRoutingPolicy,
} from '@/utils/types/chorus';
import type {
  TuttiFailoverEvent,
  TuttiFailoverPolicy,
  TuttiProxyStatus,
} from '@/utils/types/tutti';
import type { S3User } from '@/utils/types/users';

/** proxies are polled fast (health is live), the rest changes slowly */
export const STORAGES_PROXY_INTERVAL_MS = 5_000;
export const STORAGES_SLOW_INTERVAL_MS = 30_000;

interface TuttiStoragesState {
  isLoading: boolean;
  hasError: boolean;
  isLoaded: boolean;
  storages: ChorusStorage[];
  runtimeStorages: RuntimeStorage[];
  managedStoragesEnabled: boolean;
  proxies: TuttiProxyStatus[];
  users: S3User[];
  mainStorage: string;
  replications: ChorusReplication[];
  policies: TuttiFailoverPolicy[];
  events: TuttiFailoverEvent[];
  routings: (UserRoutingPolicy | BucketRoutingPolicy)[];
  /** time of the last successful proxy refresh */
  updatedAt: number | null;
  proxyTimer: ReturnType<typeof setInterval> | null;
  slowTimer: ReturnType<typeof setInterval> | null;
}

function getInitialState(): TuttiStoragesState {
  return {
    isLoading: false,
    hasError: false,
    isLoaded: false,
    storages: [],
    runtimeStorages: [],
    managedStoragesEnabled: false,
    proxies: [],
    users: [],
    mainStorage: '',
    replications: [],
    policies: [],
    events: [],
    routings: [],
    updatedAt: null,
    proxyTimer: null,
    slowTimer: null,
  };
}

/** Data behind the storages list and the storage detail page. */
export const useTuttiStoragesStore = defineStore('tuttiStorages', () => {
  const state = reactive<TuttiStoragesState>(getInitialState());
  let generation = 0;
  let proxyRequest = 0;
  let slowRequest = 0;

  const sources = computed<StoragesSources>(() => ({
    storages: state.storages,
    proxies: state.proxies,
    users: state.users,
    replications: state.replications,
    policies: state.policies,
    events: state.events,
  }));
  const rows = computed(() => StoragesPageHelper.rows(sources.value));
  const summary = computed(() =>
    StoragesPageHelper.summary(rows.value, sources.value),
  );

  async function loadProxies() {
    const current = generation;
    const request = ++proxyRequest;
    const res = await TuttiService.getProxies();

    if (current !== generation || request !== proxyRequest) return;

    state.proxies = res.proxies ?? [];
    state.updatedAt = Date.now();
  }

  async function loadSlow() {
    const current = generation;
    const request = ++slowRequest;
    const [storages, users, replications, policies, events, routings] =
      await Promise.all([
        RuntimeStorageService.list(),
        TuttiService.getUsers().catch(() => null),
        ChorusService.getReplications().catch(() => null),
        // not readable by every role: the page works without
        TuttiService.getFailoverPolicies().catch(() => null),
        TuttiService.getFailoverEvents().catch(() => null),
        ChorusService.getRoutingPolicies({
          hideUserRoutings: false,
          hideBucketRoutings: false,
        }).catch(() => null),
      ]);

    if (current !== generation || request !== slowRequest) return;

    state.runtimeStorages = storages.storages ?? [];
    state.managedStoragesEnabled = storages.managedStoragesEnabled ?? false;
    state.storages = state.runtimeStorages.map((storage) => ({
      name: storage.name,
      address: storage.address,
      isMain: storage.isMain ?? false,
      provider: StorageProvider.S3,
      credentials: [],
    }));
    state.users = users?.users ?? state.users;
    state.mainStorage =
      storages.mainStorage ?? users?.mainStorage ?? state.mainStorage;
    state.replications = replications?.replications ?? state.replications;
    state.policies = policies?.policies ?? state.policies;
    state.events = events?.events ?? state.events;
    state.routings = routings
      ? [...routings.userRoutings, ...routings.bucketRoutings]
      : state.routings;
  }

  async function refresh(showLoading = false) {
    const current = generation;

    if (showLoading) state.isLoading = true;

    try {
      await Promise.all([loadProxies(), loadSlow()]);

      if (current !== generation) return;

      state.hasError = false;
      state.isLoaded = true;
    } catch {
      if (current === generation) state.hasError = true;
    } finally {
      if (current === generation) state.isLoading = false;
    }
  }

  function isVisible(): boolean {
    return typeof document === 'undefined' || !document.hidden;
  }

  function startPolling() {
    stopPolling();
    refresh(true);
    state.proxyTimer = setInterval(() => {
      if (isVisible()) loadProxies().catch(() => undefined);
    }, STORAGES_PROXY_INTERVAL_MS);
    state.slowTimer = setInterval(() => {
      if (isVisible()) loadSlow().catch(() => undefined);
    }, STORAGES_SLOW_INTERVAL_MS);
  }

  function stopPolling() {
    generation += 1;

    if (state.proxyTimer !== null) clearInterval(state.proxyTimer);

    if (state.slowTimer !== null) clearInterval(state.slowTimer);

    state.proxyTimer = null;
    state.slowTimer = null;
  }

  function $reset() {
    stopPolling();
    Object.assign(state, getInitialState());
  }

  return {
    ...toRefs(state),
    rows,
    summary,
    refresh,
    loadSlow,
    startPolling,
    stopPolling,
    $reset,
  };
});
