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
import { useNotification } from '@clyso/clyso-ui-kit';
import { useRouter } from 'vue-router';
import { useStoreI18n } from '@/i18n/useStoreI18n';
import {
  type TuttiCoveredBucket,
  type TuttiReplicationId,
  DeletePropagationMode,
} from '@/utils/types/tuttiCore';
import { TuttiCoreService } from '@/services/TuttiCoreService';
import i18nAddReplication from '@/components/tutti/add-replication/i18nAddReplication';
import { RouteName } from '@/utils/types/router';
import { ErrorHelper } from '@/utils/helpers/ErrorHelper';
import { ReplicationNameHelper } from '@/utils/helpers/ReplicationNameHelper';
import { ReplicationSinceHelper } from '@/utils/helpers/ReplicationSinceHelper';
import { ReplicationRescanHelper } from '@/utils/helpers/ReplicationRescanHelper';
import {
  getInitialState,
  type TuttiAddReplicationState,
} from '@/stores/tuttiAddReplicationState';

export const useTuttiAddReplicationStore = defineStore(
  'tuttiAddReplication',
  () => {
    const state = reactive<TuttiAddReplicationState>(getInitialState());
    const router = useRouter();

    async function initAddReplicationPage() {
      state.isLoading = true;
      state.hasError = false;

      try {
        const { storages } = await TuttiCoreService.getStorages();

        state.storages = storages;
        prepareForm();
      } catch {
        state.hasError = true;
      } finally {
        state.isLoading = false;
      }
    }

    const { t } = useStoreI18n({
      messages: i18nAddReplication,
    });

    const hasEnoughStorages = computed(() => state.storages.length >= 2);

    /** the main storage is the usual source; one other storage is the obvious destination */
    function prepareForm() {
      state.selectedFromStorage =
        state.storages.find((storage) => storage.isMain) ??
        state.storages[0] ??
        null;

      const others = state.storages.filter(
        (storage) => storage.name !== state.selectedFromStorage?.name,
      );

      state.selectedToStorage =
        others.length === 1 ? (others[0] ?? null) : null;
    }

    async function getBucketsList() {
      if (!state.selectedFromStorage || !state.selectedToStorage) {
        throw new Error(t('initBucketsError'));
      }

      state.bucketsRequestOptions = {
        fromStorage: state.selectedFromStorage.name,
        toStorage: state.selectedToStorage.name,
        showReplicated: true,
      };

      const { buckets, replicatedBuckets, coveredBuckets } =
        await TuttiCoreService.getBucketsForReplication(
          state.bucketsRequestOptions,
        );

      state.bucketsList = buckets ?? [];
      state.replicatedBucketsList = replicatedBuckets ?? [];
      state.coveredBucketsList = coveredBuckets ?? [];

      // "all buckets" is not allowed on an edge that already has covered buckets
      if (state.coveredBucketsList.length > 0) state.isForAllBuckets = false;
    }

    function coveredReason(covered: TuttiCoveredBucket): string {
      return t(
        covered.ownerKind === 'group'
          ? 'coveredByGroup'
          : 'coveredByReplication',
        {
          id:
            covered.ownerKind === 'group'
              ? covered.ownerId
              : ReplicationNameHelper.labelFromIdString(
                  covered.ownerId,
                  t('coveredAllBuckets'),
                ),
        },
      );
    }

    /** covered buckets are shown as disabled rows with the owner as reason */
    const coveredRows = computed(() =>
      state.coveredBucketsList.map((covered) => ({
        name: covered.bucket,
        reason: coveredReason(covered),
      })),
    );
    const isAllBucketsDisabled = computed(
      () => state.coveredBucketsList.length > 0,
    );
    const allBucketsDisabledReason = computed(() => {
      const [first] = state.coveredBucketsList;

      return first ? coveredReason(first) : '';
    });

    async function initBucketsList(resetSelection = true) {
      state.isBucketsListLoading = true;

      if (resetSelection) state.selectedBuckets = [];

      state.bucketsList = [];
      state.replicatedBucketsList = [];
      state.coveredBucketsList = [];
      state.bucketsError = '';

      try {
        await getBucketsList();
        state.hasBucketsError = false;
      } catch (error) {
        state.hasBucketsError = true;
        state.bucketsError =
          ErrorHelper.getReason(error) ||
          (error instanceof Error ? error.message : t('initBucketsError'));
      } finally {
        state.isBucketsListLoading = false;
      }
    }

    const isBucketsAlreadyRequested = computed(
      () =>
        state.bucketsRequestOptions?.fromStorage ===
          state.selectedFromStorage?.name &&
        state.bucketsRequestOptions?.toStorage ===
          state.selectedToStorage?.name,
    );

    /** resolves true only when every replication was created */
    async function createReplication(): Promise<boolean> {
      const {
        selectedToStorage,
        selectedFromStorage,
        selectedBuckets,
        isForAllBuckets,
      } = state;
      const name = ReplicationNameHelper.normalize(state.name);
      // relative values count back from this click, not from when they were typed
      const since = ReplicationSinceHelper.resolve(state.since);
      const rescanInterval = ReplicationRescanHelper.toDuration(
        state.rescanMinutes,
      );

      if (
        ReplicationNameHelper.getProblem(name) ||
        ReplicationSinceHelper.getProblem(state.since) ||
        !selectedToStorage ||
        !selectedFromStorage ||
        (!selectedBuckets.length && !isForAllBuckets) ||
        (isForAllBuckets && isAllBucketsDisabled.value)
      ) {
        return false;
      }

      state.isSubmitting = true;
      state.createError = '';

      const successList: TuttiReplicationId[] = [];
      const errorList: [TuttiReplicationId, string][] = [];

      const baseReplicationId: TuttiReplicationId = {
        fromStorage: selectedFromStorage.name,
        toStorage: selectedToStorage.name,
      };

      const replicationIds: TuttiReplicationId[] = isForAllBuckets
        ? [{ ...baseReplicationId }]
        : selectedBuckets.map((bucket) => ({
            ...baseReplicationId,
            fromBucket: bucket,
            toBucket: bucket,
          }));

      const hasDeletePolicy =
        state.deleteMode !== DeletePropagationMode.IGNORE ||
        state.deleteDelaySeconds > 0 ||
        state.deleteIncludePrefixes.length > 0 ||
        state.deleteExcludePrefixes.length > 0;
      const hasIncludePatterns = state.includePatterns.length > 0;
      const hasExcludePatterns = state.excludePatterns.length > 0;
      const hasPolicies =
        hasDeletePolicy || hasIncludePatterns || hasExcludePatterns;
      const delaySec = Math.max(0, Number(state.deleteDelaySeconds) || 0);

      // One at a time: every request takes the same two storage locks (10 s acquire timeout), so parallel
      // requests would make the later ones fail with "storage is being modified; retry".
      for (const replicationId of replicationIds) {
        try {
          await TuttiCoreService.addReplication({
            id: replicationId,
            ...(name ? { name } : {}),
            ...(since || rescanInterval || hasPolicies
              ? {
                  opts: {
                    ...(since ? { since } : {}),
                    ...(rescanInterval ? { rescanInterval } : {}),
                    ...(hasDeletePolicy
                      ? {
                          deletePropagation: {
                            mode: state.deleteMode,
                            includePrefixes: state.deleteIncludePrefixes,
                            excludePrefixes: state.deleteExcludePrefixes,
                            delayBeforeDelete: `${delaySec}s`,
                          },
                        }
                      : {}),
                    ...(hasIncludePatterns
                      ? { includePatterns: state.includePatterns }
                      : {}),
                    ...(hasExcludePatterns
                      ? { excludePatterns: state.excludePatterns }
                      : {}),
                  },
                }
              : {}),
          });
          successList.push(replicationId);
        } catch (error: unknown) {
          const reason = ErrorHelper.getReason(error) || t('createFailedText');

          errorList.push([replicationId, reason]);
        }
      }

      state.isSubmitting = false;

      if (errorList.length !== 0) {
        // stay on the wizard so the draft is not lost; keep only what failed
        if (!isForAllBuckets) {
          const failed = new Set(errorList.map(([id]) => id.fromBucket));

          state.selectedBuckets = selectedBuckets.filter((bucket) =>
            failed.has(bucket),
          );
        }

        const content = errorList.map(([id, reason]) =>
          id.fromBucket ? `${id.fromBucket}: ${reason}` : reason,
        );

        state.createError = content.join('; ');
      } else {
        router.push({ name: RouteName.TUTTI_REPLICATION });
      }

      if (successList.length !== 0) {
        showCreateSuccess();
      }

      return errorList.length === 0 && successList.length > 0;
    }

    const { createNotification } = useNotification();

    function showCreateSuccess() {
      createNotification({
        type: 'success',
        title: t('createdTitle'),
        isClosable: true,
      });
    }

    function $reset() {
      Object.assign(state, getInitialState());
    }

    return {
      ...toRefs(state),
      hasEnoughStorages,
      initBucketsList,
      getBucketsList,
      isBucketsAlreadyRequested,
      coveredRows,
      isAllBucketsDisabled,
      allBucketsDisabledReason,
      createReplication,
      $reset,
      initAddReplicationPage,
    };
  },
);
