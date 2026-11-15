/*
 * Copyright © 2026 Clyso GmbH
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
import { usePolling } from '@/utils/composables/usePolling';
import { createChangeDetector } from '@/utils/helpers/changeDetector';
import { useStoreI18n } from '@/i18n/useStoreI18n';
import { GeneralHelper } from '@/utils/helpers/GeneralHelper';
import {
  RoutingPolicyStatusFilter,
  RoutingPolicyTypes,
  type RoutingPolicy,
  type RoutingPolicyEditRequest,
} from '@/utils/types/tuttiCore';
import { TuttiCoreService } from '@/services/TuttiCoreService';
import { ErrorHelper } from '@/utils/helpers/ErrorHelper';
import i18nRouting from '@/pages/TuttiRoutingPage/i18nRouting';

interface TuttiRoutingPoliciesState {
  isLoading: boolean;
  hasError: boolean;
  routingPolicies: RoutingPolicy[];

  filterUsers: string[];
  filterBuckets: string[];
  filterStorages: string[];
  filterStatus: RoutingPolicyStatusFilter | null;
}

const POLL_INTERVAL_MS = 5000;

function getInitialState(): TuttiRoutingPoliciesState {
  return {
    isLoading: false,
    hasError: false,
    routingPolicies: [],

    filterUsers: [],
    filterBuckets: [],
    filterStorages: [],
    filterStatus: null,
  };
}

export const useTuttiRoutingPoliciesStore = defineStore(
  'tuttiRoutingPolicies',
  () => {
    const state = reactive<TuttiRoutingPoliciesState>(getInitialState());
    const polling = usePolling();
    const changes = createChangeDetector();

    const { t } = useStoreI18n({
      messages: i18nRouting,
    });

    const isIn = (list: string[], value: string) =>
      list.length === 0 || list.includes(value);

    const filteredRoutingPolicies = computed<RoutingPolicy[]>(() =>
      state.routingPolicies.filter(
        (rule) =>
          isIn(state.filterUsers, rule.user) &&
          isIn(state.filterStorages, rule.toStorage) &&
          isIn(state.filterBuckets, rule.bucket) &&
          (state.filterStatus === null ||
            (state.filterStatus === RoutingPolicyStatusFilter.BLOCKED) ===
              rule.isBlocked),
      ),
    );

    /** filtered rows, in a stable order (the kit table pages them itself) */
    const computedRoutingPolicies = computed<RoutingPolicy[]>(() =>
      GeneralHelper.orderBy(filteredRoutingPolicies.value, ['id'], ['asc']),
    );

    const hasNoData = computed<boolean>(
      () => state.routingPolicies.length === 0,
    );

    const isFiltered = computed<boolean>(
      () =>
        state.filterUsers.length !== 0 ||
        state.filterBuckets.length !== 0 ||
        state.filterStorages.length !== 0 ||
        state.filterStatus !== null,
    );

    function clearFilters() {
      state.filterUsers = [];
      state.filterBuckets = [];
      state.filterStorages = [];
      state.filterStatus = null;
    }

    async function getRoutingPolicies() {
      const res = await TuttiCoreService.getRoutingPolicies({
        hideUserRoutings: false,
        hideBucketRoutings: false,
      });

      if (!changes.hasChanged(res)) return;

      const routingPolicies: RoutingPolicy[] = [
        ...res.userRoutings.map((userPolicy) => ({
          ...userPolicy,
          bucket: '*',
          type: RoutingPolicyTypes.USER,
          id: `${userPolicy.toStorage}::${userPolicy.user}`,
        })),
        ...res.bucketRoutings.map((bucketPolicy) => ({
          ...bucketPolicy,
          type: RoutingPolicyTypes.BUCKET,
          id: `${bucketPolicy.toStorage}::${bucketPolicy.user}::${bucketPolicy.bucket}`,
        })),
      ];

      state.routingPolicies = routingPolicies;
    }

    async function startRoutingPoliciesPolling() {
      changes.reset();
      await polling.start(getRoutingPolicies, POLL_INTERVAL_MS);
    }

    async function stopRoutingPoliciesPolling() {
      await polling.stop();
    }

    async function initRoutingPoliciesPage() {
      state.isLoading = true;

      try {
        await startRoutingPoliciesPolling();

        state.hasError = false;
      } catch {
        state.hasError = true;
        await stopRoutingPoliciesPolling();
      } finally {
        state.isLoading = false;
      }
    }

    /** the API addresses a rule by user and bucket; "all buckets" is null */
    const toRequest = (rule: RoutingPolicy): RoutingPolicyEditRequest => ({
      user: rule.user,
      bucket: rule.bucket === '*' ? null : rule.bucket,
    });

    const reasonOf = (error: unknown) =>
      ErrorHelper.getReason(error) || t('unknownError');

    /** runs `action` for every rule; failures are collected, not thrown */
    async function forEachRule(
      rules: RoutingPolicy[],
      action: (rule: RoutingPolicy) => Promise<void>,
    ) {
      const successList: RoutingPolicy[] = [];
      const errorList: [RoutingPolicy, string][] = [];

      await Promise.all(
        rules.map(async (rule) => {
          try {
            await action(rule);
            successList.push(rule);
          } catch (error: unknown) {
            errorList.push([
              rule,
              error instanceof Error ? error.message : String(error),
            ]);
          }
        }),
      );

      return { successList, errorList };
    }

    /** Changes the target storage; rethrows the original error for the caller. */
    async function updateRoutingPolicyTarget(
      routingPolicy: RoutingPolicy,
      toStorage: string,
    ) {
      await TuttiCoreService.updateRoutingPolicy({
        ...toRequest(routingPolicy),
        toStorage,
      });

      try {
        await getRoutingPolicies();
      } catch {
        // the next poll refreshes the list
      }
    }

    async function deleteRoutingPolicies(rules: RoutingPolicy[]) {
      await stopRoutingPoliciesPolling();

      const result = await forEachRule(rules, async (rule) => {
        try {
          await TuttiCoreService.deleteRoutingPolicy(toRequest(rule));
        } catch (error: unknown) {
          throw new Error(reasonOf(error));
        }
      });
      const removed = new Set(result.successList.map((rule) => rule.id));

      state.routingPolicies = state.routingPolicies.filter(
        (rule) => !removed.has(rule.id),
      );
      startRoutingPoliciesPolling();

      return result;
    }

    async function setRoutingPoliciesBlock(
      rules: RoutingPolicy[],
      setBlock: boolean,
    ) {
      await stopRoutingPoliciesPolling();

      const call = setBlock
        ? TuttiCoreService.blockRoutingPolicy
        : TuttiCoreService.unblockRoutingPolicy;
      const result = await forEachRule(rules, async (rule) => {
        try {
          await call(toRequest(rule));
        } catch (error: unknown) {
          throw new Error(reasonOf(error));
        }

        const index = state.routingPolicies.findIndex(
          ({ id }) => id === rule.id,
        );

        if (index !== -1)
          state.routingPolicies.splice(index, 1, {
            ...rule,
            isBlocked: setBlock,
          });
      });

      startRoutingPoliciesPolling();

      return result;
    }

    async function $reset() {
      try {
        await stopRoutingPoliciesPolling();
      } finally {
        Object.assign(state, getInitialState());
      }
    }

    return {
      ...toRefs(state),
      hasNoData,
      updateRoutingPolicyTarget,
      deleteRoutingPolicies,
      setRoutingPoliciesBlock,
      computedRoutingPolicies,
      initRoutingPoliciesPage,
      isFiltered,
      clearFilters,
      $reset,
    };
  },
);
