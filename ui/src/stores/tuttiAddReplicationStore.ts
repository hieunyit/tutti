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
import type { Step } from '@clyso/clyso-ui-kit';
import useVuelidate from '@vuelidate/core';
import { useNotification } from '@clyso/clyso-ui-kit';
import { useRouter } from 'vue-router';
import { useStoreI18n } from '@/i18n/useStoreI18n';
import {
  AddReplicationStepName,
  type ChorusBucketListRequest,
  type ChorusCoveredBucket,
  type ChorusReplicationId,
  type ChorusStorage,
} from '@/utils/types/chorus';
import { ChorusService } from '@/services/ChorusService';
import i18nAddReplication from '@/components/chorus/add-replication/i18nAddReplication';
import { RouteName } from '@/utils/types/router';
import { ErrorHelper } from '@/utils/helpers/ErrorHelper';

interface ChorusAddReplicationState {
  isLoading: boolean;
  hasError: boolean;
  storages: ChorusStorage[];
  isMainAsSourceAlertShown: boolean;
  selectedFromStorage: ChorusStorage | null;
  selectedToStorage: ChorusStorage | null;
  isForAllBuckets: boolean;
  isBucketsListLoading: boolean;
  hasBucketsError: boolean;
  bucketsError: string;
  isReplicatedShown: boolean;
  bucketsRequestOptions: ChorusBucketListRequest | null;
  bucketsList: string[];
  replicatedBucketsList: string[];
  coveredBucketsList: ChorusCoveredBucket[];
  /** server text of the last failed create, shown on the page */
  createError: string;
  selectedBuckets: string[];
  currentStep: AddReplicationStepName;
  isConfirmDialogOpen: boolean;
  isSubmitting: boolean;
}

function getInitialState(): ChorusAddReplicationState {
  return {
    isLoading: false,
    hasError: false,
    storages: [],
    isMainAsSourceAlertShown: false,
    selectedFromStorage: null,
    selectedToStorage: null,
    isForAllBuckets: false,
    isBucketsListLoading: false,
    hasBucketsError: false,
    bucketsError: '',
    isReplicatedShown: false,
    bucketsRequestOptions: null,
    bucketsList: [],
    replicatedBucketsList: [],
    coveredBucketsList: [],
    createError: '',
    selectedBuckets: [],
    currentStep: AddReplicationStepName.FROM_STORAGE,
    isConfirmDialogOpen: false,
    isSubmitting: false,
  };
}

export const useTuttiAddReplicationStore = defineStore(
  'tuttiAddReplication',
  () => {
    const state = reactive<ChorusAddReplicationState>(getInitialState());
    const router = useRouter();

    async function initAddReplicationPage() {
      state.isLoading = true;
      state.hasError = false;

      try {
        const { storages } = await ChorusService.getStorages();

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

    const steps = computed<Step[]>(() => [
      {
        title: t('step1Title'),
        description: t('step1Description'),
      },
      {
        title: t('step2Title'),
        description: t('step2Description'),
      },
      {
        title: t('step3Title'),
        description: t('step3Description'),
      },
    ]);
    const stepsCount = computed(() => steps.value.length);

    function prepareForm() {
      state.selectedFromStorage = state.storages[0] ?? null;
      state.selectedToStorage =
        state.storages.find(
          (s) => s.name !== state.selectedFromStorage?.name,
        ) ??
        state.storages[1] ??
        null;
      state.isMainAsSourceAlertShown = false;
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
        await ChorusService.getBucketsForReplication(
          state.bucketsRequestOptions,
        );

      state.bucketsList = buckets ?? [];
      state.replicatedBucketsList = replicatedBuckets ?? [];
      state.coveredBucketsList = coveredBuckets ?? [];

      // "all buckets" is not allowed on an edge that already has covered buckets
      if (state.coveredBucketsList.length > 0) state.isForAllBuckets = false;
    }

    function coveredReason(covered: ChorusCoveredBucket): string {
      return t('coveredReason', {
        kind: covered.ownerKind,
        id: covered.ownerId,
      });
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

    const validator = useVuelidate(
      {
        state: {
          selectedBuckets: {
            required: () => {
              if (state.isForAllBuckets) {
                return true;
              }

              return state.selectedBuckets.length > 0;
            },
          },
        },
      },
      {
        state,
      },
    );

    /** resolves true only when every replication was created */
    async function createReplication(): Promise<boolean> {
      const {
        selectedToStorage,
        selectedFromStorage,
        selectedBuckets,
        isForAllBuckets,
      } = state;

      if (
        !selectedToStorage ||
        !selectedFromStorage ||
        (!selectedBuckets.length && !isForAllBuckets) ||
        (isForAllBuckets && isAllBucketsDisabled.value)
      ) {
        return false;
      }

      state.isSubmitting = true;
      state.createError = '';

      const successList: ChorusReplicationId[] = [];
      const errorList: [ChorusReplicationId, string][] = [];

      const baseReplicationId: ChorusReplicationId = {
        fromStorage: selectedFromStorage.name,
        toStorage: selectedToStorage.name,
      };

      const replicationIds: ChorusReplicationId[] = isForAllBuckets
        ? [{ ...baseReplicationId }]
        : selectedBuckets.map((bucket) => ({
            ...baseReplicationId,
            fromBucket: bucket,
            toBucket: bucket,
          }));

      // One at a time: every request takes the same two storage locks (10 s acquire timeout), so parallel
      // requests would make the later ones fail with "storage is being modified; retry".
      for (const replicationId of replicationIds) {
        try {
          await ChorusService.addReplication({
            id: replicationId,
          });
          successList.push(replicationId);
        } catch (error: unknown) {
          const reason =
            ErrorHelper.getReason(error) || t('createReplicationErrorUnknown');

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
        showCreateError(state.createError);
      } else {
        router.push({ name: RouteName.CHORUS_REPLICATION });
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
        title: `${t('successTitle')}`,
        content: t('createReplicationSuccess'),
        isClosable: true,
      });
    }

    function showCreateError(reason?: string) {
      const content = reason ? reason : t('createReplicationError');

      createNotification({
        type: 'error',
        title: `${t('errorTitle')}`,
        content: content,
        isClosable: true,
      });
    }

    function $reset() {
      Object.assign(state, getInitialState());
      validator.value.$reset();
    }

    return {
      ...toRefs(state),
      steps,
      stepsCount,
      initBucketsList,
      getBucketsList,
      isBucketsAlreadyRequested,
      coveredRows,
      isAllBucketsDisabled,
      allBucketsDisabledReason,
      validator,
      createReplication,
      $reset,
      initAddReplicationPage,
    };
  },
);
