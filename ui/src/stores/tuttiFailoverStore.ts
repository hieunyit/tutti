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
import { FailoverHelper } from '@/utils/helpers/FailoverHelper';
import type {
  TuttiFailoverEvent,
  TuttiFailoverPolicy,
  TuttiFailoverPolicyForm,
  TuttiProxyStatus,
} from '@/utils/types/tutti';

interface TuttiFailoverState {
  isLoading: boolean;
  hasError: boolean;
  policies: TuttiFailoverPolicy[];
  events: TuttiFailoverEvent[];
  proxies: TuttiProxyStatus[];
  storageNames: string[];
  isSaving: boolean;
  saveError: string | null;
}

function getInitialState(): TuttiFailoverState {
  return {
    isLoading: false,
    hasError: false,
    policies: [],
    events: [],
    proxies: [],
    storageNames: [],
    isSaving: false,
    saveError: null,
  };
}

export const useTuttiFailoverStore = defineStore('tuttiFailover', () => {
  const state = reactive<TuttiFailoverState>(getInitialState());

  const proxiesWithFailoverDisabled = computed(
    () =>
      state.proxies.filter(
        (proxy) => proxy.settings?.failover?.enabled !== true,
      ).length,
  );

  async function init() {
    state.isLoading = true;
    state.hasError = false;

    try {
      const [policies, events, proxies, storages] = await Promise.all([
        TuttiService.getFailoverPolicies(),
        TuttiService.getFailoverEvents(),
        TuttiService.getProxies(),
        ChorusService.getStorages(),
      ]);

      state.policies = policies.policies ?? [];
      state.events = events.events ?? [];
      state.proxies = proxies.proxies ?? [];
      state.storageNames = (storages.storages ?? []).map((s) => s.name);
    } catch {
      state.hasError = true;
    } finally {
      state.isLoading = false;
    }
  }

  async function reloadPolicies() {
    const res = await TuttiService.getFailoverPolicies();

    state.policies = res.policies ?? [];
  }

  async function reloadEvents() {
    const res = await TuttiService.getFailoverEvents();

    state.events = res.events ?? [];
  }

  /** Returns true on success; on failure sets saveError (backend message if validation error). */
  async function savePolicy(form: TuttiFailoverPolicyForm): Promise<boolean> {
    state.isSaving = true;
    state.saveError = null;

    try {
      await TuttiService.setFailoverPolicy(FailoverHelper.formToPolicy(form));
      await reloadPolicies();

      return true;
    } catch (e) {
      state.saveError = FailoverHelper.getValidationMessage(e) ?? 'unknown';

      return false;
    } finally {
      state.isSaving = false;
    }
  }

  async function deletePolicy(source: string): Promise<boolean> {
    try {
      await TuttiService.deleteFailoverPolicy(source);
      await reloadPolicies();

      return true;
    } catch {
      return false;
    }
  }

  async function acknowledgeEvent(event: TuttiFailoverEvent): Promise<boolean> {
    try {
      await TuttiService.deleteFailoverEvent({
        user: event.user,
        bucket: event.bucket ?? '',
      });
      await reloadEvents();

      return true;
    } catch {
      return false;
    }
  }

  function $reset() {
    Object.assign(state, getInitialState());
  }

  return {
    ...toRefs(state),
    proxiesWithFailoverDisabled,
    init,
    savePolicy,
    deletePolicy,
    acknowledgeEvent,
    $reset,
  };
});
