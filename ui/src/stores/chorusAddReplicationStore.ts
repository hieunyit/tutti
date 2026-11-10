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
  type ChorusReplicationId,
  type ChorusStorage,
} from '@/utils/types/chorus';
import { ChorusService } from '@/services/ChorusService';
import { TuttiService } from '@/services/TuttiService';
import type { S3User } from '@/utils/types/users';
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
  selectedUser: string | null;
  /** S3 users (replications are per user), loaded from /tutti/users */
  users: S3User[];
  hasUsersError: boolean;
  isForAllBuckets: boolean;
  isBucketsListLoading: boolean;
  hasBucketsError: boolean;
  isReplicatedShown: boolean;
  bucketsRequestOptions: ChorusBucketListRequest | null;
  bucketsList: string[];
  replicatedBucketsList: string[];
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
    selectedUser: null,
    users: [],
    hasUsersError: false,
    isForAllBuckets: false,
    isBucketsListLoading: false,
    hasBucketsError: false,
    isReplicatedShown: false,
    bucketsRequestOptions: null,
    bucketsList: [],
    replicatedBucketsList: [],
    selectedBuckets: [],
    currentStep: AddReplicationStepName.FROM_STORAGE,
    isConfirmDialogOpen: false,
    isSubmitting: false,
  };
}

export const useChorusAddReplicationStore = defineStore(
  'chorusAddReplication',
  () => {
    const state = reactive<ChorusAddReplicationState>(getInitialState());
    const router = useRouter();

    async function initAddReplicationPage() {
      state.isLoading = true;
      state.hasError = false;

      try {
        const [{ storages }, usersResponse] = await Promise.all([
          ChorusService.getStorages(),
          TuttiService.getUsers().catch(() => null),
        ]);

        state.storages = storages;
        state.users = usersResponse?.users ?? [];
        state.hasUsersError = usersResponse === null;
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
      {
        title: t('step4Title'),
        description: t('step4Description'),
      },
    ]);
    const stepsCount = computed(() => steps.value.length);

    function prepareForm() {
      state.selectedFromStorage =
        state.storages.find((storage) => storage.isMain) ?? null;
      state.selectedToStorage =
        state.storages.find((storage) => !storage.isMain) ?? null;
      // the user is chosen explicitly: never default to an arbitrary tenant
      state.selectedUser = null;
      setTimeout(() => {
        state.isMainAsSourceAlertShown = true;
      }, 500);
    }

    async function getBucketsList() {
      if (
        !state.selectedUser ||
        !state.selectedFromStorage ||
        !state.selectedToStorage
      ) {
        return;
      }

      state.bucketsRequestOptions = {
        user: state.selectedUser,
        fromStorage: state.selectedFromStorage.name,
        toStorage: state.selectedToStorage.name,
        showReplicated: true,
      };

      const { buckets, replicatedBuckets } =
        await ChorusService.getBucketsForReplication(
          state.bucketsRequestOptions,
        );

      state.bucketsList = buckets;
      state.replicatedBucketsList = replicatedBuckets;
    }

    async function initBucketsList() {
      state.isBucketsListLoading = true;
      state.selectedBuckets = [];
      state.bucketsList = [];
      state.replicatedBucketsList = [];

      try {
        await getBucketsList();
        state.hasBucketsError = false;
      } catch {
        state.hasBucketsError = true;
      } finally {
        state.isBucketsListLoading = false;
      }
    }

    const isBucketsAlreadyRequested = computed(
      () =>
        state.bucketsRequestOptions?.user === state.selectedUser &&
        state.bucketsRequestOptions?.fromStorage ===
          state.selectedFromStorage?.name &&
        state.bucketsRequestOptions?.toStorage ===
          state.selectedToStorage?.name,
    );

    /** how the chosen user is signed on a storage: override, connection-account, missing or unknown */
    function userModeOn(user: string, storage: string | undefined): string {
      const entry = state.users
        .find((u) => u.name === user)
        ?.storages?.find((s) => s.storage === storage);

      if (!entry) {
        return 'unknown';
      }

      return entry.mode || (entry.configured ? 'override' : 'missing');
    }

    /** the selected user has usable credentials on both chosen storages */
    const isSelectedUserUsable = computed(() => {
      if (!state.selectedUser) {
        return false;
      }

      return [
        state.selectedFromStorage?.name,
        state.selectedToStorage?.name,
      ].every(
        (storage) =>
          userModeOn(state.selectedUser as string, storage) !== 'missing',
      );
    });

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

    async function createReplication() {
      const {
        selectedToStorage,
        selectedFromStorage,
        selectedUser,
        selectedBuckets,
        isForAllBuckets,
      } = state;

      if (
        !selectedToStorage ||
        !selectedFromStorage ||
        !selectedUser ||
        (!selectedBuckets.length && !isForAllBuckets)
      ) {
        return;
      }

      state.isSubmitting = true;

      const successList: ChorusReplicationId[] = [];
      const errorList: [ChorusReplicationId, string][] = [];

      const baseReplicationId: ChorusReplicationId = {
        user: selectedUser,
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

      await Promise.all(
        replicationIds.map(async (replicationId) => {
          try {
            await ChorusService.addReplication({
              id: replicationId,
            });
            successList.push(replicationId);
          } catch (error: unknown) {
            const reason =
              ErrorHelper.getReason(error) ||
              t('createReplicationErrorUnknown');

            errorList.push([replicationId, reason]);
          }
        }),
      );

      router.push({ name: RouteName.CHORUS_REPLICATION });
      state.isSubmitting = false;

      if (errorList.length !== 0) {
        const content = errorList.map(([id, reason]) => {
          const bucketInfo = id.fromBucket ? ` (${id.fromBucket})` : '';

          return `${id.user}/${id.fromStorage}->${id.toStorage}${bucketInfo}: ${reason}`;
        });

        showCreateError(content ? content.join(', ') : undefined);
      }

      if (successList.length !== 0) {
        showCreateSuccess();
      }
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
      userModeOn,
      isSelectedUserUsable,
      validator,
      createReplication,
      $reset,
      initAddReplicationPage,
    };
  },
);
