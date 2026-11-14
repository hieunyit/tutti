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
import { computed, h, reactive, toRefs } from 'vue';
import type {
  DataTablePaginationObject,
  DataTableSortState,
} from '@clyso/clyso-ui-kit';
import { useNotification } from '@clyso/clyso-ui-kit';
import { usePolling } from '@/utils/composables/usePolling';
import { useStoreI18n } from '@/i18n/useStoreI18n';
import type {
  TuttiReplication,
  TuttiReplicationId,
} from '@/utils/types/tuttiCore';
import {
  ReplicationStatusFilter,
  ReplicationType,
} from '@/utils/types/tuttiCore';
import { GeneralHelper } from '@/utils/helpers/GeneralHelper';
import type { AddId } from '@/utils/types/helper';
import { TuttiCoreService } from '@/services/TuttiCoreService';
import ReplicationsShortList from '@/components/tutti/replications/ReplicationsShortList/ReplicationsShortList.vue';
import i18nReplications from '@/components/tutti/replications/i18nReplications';
import { createChangeDetector } from '@/utils/helpers/changeDetector';
import { ReplicationsHelper } from '@/utils/helpers/ReplicationsHelper';

interface TuttiReplicationsState {
  isLoading: boolean;
  hasError: boolean;
  replications: AddId<TuttiReplication>[];
  sorter: DataTableSortState | null;
  page: number;
  pageSize: number;

  selectedReplicationIds: string[];
  isResumeSelectedProcessing: boolean;
  isPauseSelectedProcessing: boolean;
  isDeleteSelectedProcessing: boolean;

  filterUsers: string[];
  filterBuckets: string[];
  filterDirections: string[];
  filterStatuses: ReplicationStatusFilter[];
  filterCreatedAtRange: [number, number] | null;
  filterType: ReplicationType | null;
}

const PAGE_SIZES = [10, 20, 30, 50, 100] as const;
const POLL_INTERVAL_MS = 5000;

function getTuttiReplicationId(replicationId: TuttiReplicationId): string {
  return (
    `${replicationId.user}${replicationId.fromBucket}` +
    `${replicationId.toBucket}${replicationId.fromStorage}` +
    `${replicationId.toStorage}`
  );
}

function getReplicationType(
  replicationId: TuttiReplicationId,
): ReplicationType {
  return replicationId.fromBucket && replicationId.toBucket
    ? ReplicationType.BUCKET
    : ReplicationType.USER;
}

function getInitialState(): TuttiReplicationsState {
  return {
    isLoading: false,
    hasError: false,
    replications: [],
    sorter: null,
    page: 1,
    pageSize: PAGE_SIZES[0],
    selectedReplicationIds: [],
    isResumeSelectedProcessing: false,
    isPauseSelectedProcessing: false,
    isDeleteSelectedProcessing: false,

    filterUsers: [],
    filterBuckets: [],
    filterDirections: [],
    filterStatuses: [],
    filterCreatedAtRange: null,
    filterType: null,
  };
}

export const useTuttiReplicationsStore = defineStore(
  'tuttiReplications',
  () => {
    const state = reactive<TuttiReplicationsState>(getInitialState());
    const polling = usePolling();
    const changes = createChangeDetector();

    const { createNotification, removeNotification } = useNotification();

    const { t } = useStoreI18n({
      messages: i18nReplications,
    });

    const hasNoData = computed<boolean>(() => state.replications.length === 0);

    const filteredReplications = computed<AddId<TuttiReplication>[]>(() =>
      state.replications.filter((replication) => {
        const isUserMatched =
          !state.filterUsers.length ||
          state.filterUsers.includes(replication.id.user ?? '');
        const isBucketMatched =
          !state.filterBuckets.length ||
          state.filterBuckets.includes(
            ReplicationsHelper.getBucketPairString(replication),
          );
        const isDirectionsMatched =
          !state.filterDirections.length ||
          state.filterDirections.includes(
            ReplicationsHelper.getDirectionPairString(replication),
          );
        const isStatusMatched =
          !state.filterStatuses.length ||
          state.filterStatuses.every((status) =>
            ReplicationsHelper.isReplicationStatusMatched(replication, status),
          );
        const isCreatedAtMatched =
          !state.filterCreatedAtRange ||
          ReplicationsHelper.isReplicationCreateAtMatched(
            replication,
            state.filterCreatedAtRange,
          );
        const isTypeMatched =
          !state.filterType || replication.replicationType === state.filterType;

        return (
          isUserMatched &&
          isBucketMatched &&
          isDirectionsMatched &&
          isStatusMatched &&
          isCreatedAtMatched &&
          isTypeMatched
        );
      }),
    );

    const isFiltered = computed<boolean>(
      () =>
        state.filterUsers.length !== 0 ||
        state.filterBuckets.length !== 0 ||
        state.filterDirections.length !== 0 ||
        state.filterStatuses.length !== 0 ||
        state.filterCreatedAtRange !== null ||
        state.filterType !== null,
    );

    function clearFilters() {
      state.filterUsers = [];
      state.filterBuckets = [];
      state.filterDirections = [];
      state.filterStatuses = [];
      state.filterCreatedAtRange = null;
      state.filterType = null;
    }

    const computedReplications = computed<AddId<TuttiReplication>[]>(() => {
      const pageReplications = state.sorter
        ? GeneralHelper.orderBy(
            filteredReplications.value,
            [state.sorter.columnKey],
            [state.sorter.order === 'ascend' ? 'asc' : 'desc'],
          )
        : filteredReplications.value;

      const start = (state.page - 1) * state.pageSize;
      const end = state.page * state.pageSize;

      return pageReplications.slice(start, end);
    });

    const pagination = computed<DataTablePaginationObject>(() => ({
      page: state.page,
      pageSize: state.pageSize,
      showSizePicker: true,
      pageSizes: [...PAGE_SIZES],
      pageCount: Math.ceil(filteredReplications.value.length / state.pageSize),
      itemCount: filteredReplications.value.length,
      prefix({ itemCount }) {
        if (state.isLoading || state.hasError) {
          return '';
        }

        if (isFiltered.value) {
          return `Filtered: ${filteredReplications.value.length} / Total: ${state.replications.length}`;
        }

        return `Total: ${itemCount}`;
      },
    }));

    async function getReplications() {
      const res = await TuttiCoreService.getReplications();

      if (!changes.hasChanged(res)) return;

      state.replications = (res.replications ?? []).map((replication) => ({
        ...replication,
        idStr: getTuttiReplicationId(replication.id),
        replicationType: getReplicationType(replication.id),
      }));
    }

    async function startReplicationPolling() {
      changes.reset();
      await polling.start(getReplications, POLL_INTERVAL_MS);
    }

    async function stopReplicationPolling() {
      await polling.stop();
    }

    async function initReplicationsPage() {
      state.isLoading = true;

      try {
        await startReplicationPolling();

        state.hasError = false;
      } catch {
        state.hasError = true;
        await stopReplicationPolling();
      } finally {
        state.isLoading = false;
      }
    }

    function setReplicationById(
      id: string | number,
      partialReplication: Partial<AddId<TuttiReplication>>,
    ) {
      const index = state.replications.findIndex(
        (replication) => replication.idStr === id,
      );

      if (index === -1) {
        return;
      }

      const matchedReplication = state.replications[index];

      if (!matchedReplication) {
        return;
      }

      state.replications.splice(index, 1, {
        ...matchedReplication,
        ...partialReplication,
      });
    }

    async function setReplicationPaused(
      replication: AddId<TuttiReplication>,
      isPaused: boolean,
    ) {
      const replicationIndex = state.replications.findIndex(
        ({ id }) => replication.id === id,
      );

      if (replicationIndex === -1) {
        return;
      }

      const { user, fromBucket, toBucket, fromStorage, toStorage } =
        replication.id;

      await stopReplicationPolling();
      await (
        isPaused
          ? TuttiCoreService.pauseBucketReplication
          : TuttiCoreService.resumeBucketReplication
      )({
        user,
        fromBucket,
        toBucket,
        fromStorage,
        toStorage,
      });

      state.replications.splice(replicationIndex, 1, {
        ...replication,
        isPaused,
      });
      startReplicationPolling();
    }

    async function deleteReplication(replication: AddId<TuttiReplication>) {
      let replicationIndex = state.replications.findIndex(
        ({ id }) => replication.id === id,
      );

      if (replicationIndex === -1) {
        return;
      }

      await stopReplicationPolling();
      await TuttiCoreService.deleteReplication(replication.id);

      replicationIndex = state.replications.findIndex(
        ({ id }) => replication.id === id,
      );

      state.replications.splice(replicationIndex, 1);
      startReplicationPolling();
    }

    const selectedReplicationsCount = computed(
      () => state.selectedReplicationIds.length,
    );
    const isAnyReplicationsSelected = computed(
      () => state.selectedReplicationIds.length !== 0,
    );
    const selectedReplications = computed<AddId<TuttiReplication>[]>(() =>
      state.replications.filter((replication) =>
        state.selectedReplicationIds.includes(replication.idStr),
      ),
    );

    const isSelectedProcessing = computed(
      () =>
        state.isResumeSelectedProcessing ||
        state.isPauseSelectedProcessing ||
        state.isDeleteSelectedProcessing,
    );

    async function resumeReplications(replications: AddId<TuttiReplication>[]) {
      state.isResumeSelectedProcessing = true;

      const successList: AddId<TuttiReplication>[] = [];
      const errorList: AddId<TuttiReplication>[] = [];

      await Promise.all(
        replications.map(async (replication) => {
          const { fromStorage, toStorage, user, fromBucket, toBucket } =
            replication.id;

          try {
            await TuttiCoreService.resumeBucketReplication({
              fromStorage,
              toStorage,
              user,
              fromBucket,
              toBucket,
            });
            successList.push(replication);
          } catch {
            errorList.push(replication);
          }
        }),
      );

      if (errorList.length !== 0) {
        const errorNotification = createNotification({
          type: 'error',
          title: t('resumeErrorTitle'),
          positiveText: t('errorAction'),
          positiveHandler: () => {
            removeNotification(errorNotification.value.id);
            resumeReplications(errorList);
          },
          content: () =>
            h('div', [
              t('resumeSelectedErrorContent', { total: errorList.length }),
              h(ReplicationsShortList, {
                replications: errorList,
              }),
            ]),
        });
      }

      if (successList.length !== 0) {
        successList.forEach((replication) => {
          setReplicationById(replication.idStr, { isPaused: false });
        });
        createNotification({
          type: 'success',
          title: t('resumeSuccessTitle'),
          duration: 4000,
          content: () =>
            h('div', [
              t('resumeSelectedSuccessContent', { total: successList.length }),
              h(ReplicationsShortList, {
                replications: successList,
              }),
            ]),
        });
      }

      startReplicationPolling();
      state.isResumeSelectedProcessing = false;
    }

    async function pauseReplications(replications: AddId<TuttiReplication>[]) {
      state.isPauseSelectedProcessing = true;

      const successList: AddId<TuttiReplication>[] = [];
      const errorList: AddId<TuttiReplication>[] = [];

      await Promise.all(
        replications.map(async (replication) => {
          const { fromStorage, toStorage, user, fromBucket, toBucket } =
            replication.id;

          try {
            await TuttiCoreService.pauseBucketReplication({
              fromStorage,
              toStorage,
              user,
              fromBucket,
              toBucket,
            });
            successList.push(replication);
          } catch {
            errorList.push(replication);
          }
        }),
      );

      if (errorList.length !== 0) {
        const errorNotification = createNotification({
          type: 'error',
          title: t('pauseErrorTitle'),
          positiveText: t('errorAction'),
          positiveHandler: () => {
            removeNotification(errorNotification.value.id);
            pauseReplications(errorList);
          },
          content: () =>
            h('div', [
              t('pauseSelectedErrorContent', { total: errorList.length }),
              h(ReplicationsShortList, {
                replications: errorList,
              }),
            ]),
        });
      }

      if (successList.length !== 0) {
        successList.forEach((replication) => {
          setReplicationById(replication.idStr, { isPaused: true });
        });
        createNotification({
          type: 'success',
          title: t('pauseSuccessTitle'),
          duration: 4000,
          content: () =>
            h('div', [
              t('pauseSelectedSuccessContent', { total: successList.length }),
              h(ReplicationsShortList, {
                replications: successList,
              }),
            ]),
        });
      }

      startReplicationPolling();
      state.isPauseSelectedProcessing = false;
    }

    async function deleteReplications(replications: AddId<TuttiReplication>[]) {
      state.isDeleteSelectedProcessing = true;

      const successList: AddId<TuttiReplication>[] = [];
      const errorList: AddId<TuttiReplication>[] = [];

      await Promise.all(
        replications.map(async (replication) => {
          try {
            await TuttiCoreService.deleteReplication(replication.id);
            successList.push(replication);
          } catch {
            errorList.push(replication);
          }
        }),
      );

      if (errorList.length !== 0) {
        const errorNotification = createNotification({
          type: 'error',
          title: t('deleteErrorTitle'),
          positiveText: t('errorAction'),
          positiveHandler: () => {
            removeNotification(errorNotification.value.id);
            deleteReplications(errorList);
          },
          content: () =>
            h('div', [
              t('deleteSelectedErrorContent', { total: errorList.length }),
              h(ReplicationsShortList, {
                replications: errorList,
              }),
            ]),
        });
      }

      if (successList.length !== 0) {
        const successListIds = successList.map((item) => item.idStr);

        state.replications = state.replications.filter(
          (item) => !successListIds.includes(item.idStr),
        );
        state.selectedReplicationIds = state.selectedReplicationIds.filter(
          (item) => !successListIds.includes(item),
        );

        const pageCount = pagination.value.pageCount ?? 1;

        if (state.page > pageCount) {
          state.page = pageCount;
        }

        createNotification({
          type: 'success',
          title: t('deleteSuccessTitle'),
          duration: 4000,
          content: () =>
            h('div', [
              t('deleteSelectedSuccessContent', { total: successList.length }),
              h(ReplicationsShortList, {
                replications: successList,
              }),
            ]),
        });
      }

      startReplicationPolling();
      state.isDeleteSelectedProcessing = false;
    }

    async function $reset() {
      try {
        await stopReplicationPolling();
      } finally {
        Object.assign(state, getInitialState());
      }
    }

    return {
      ...toRefs(state),
      hasNoData,
      pagination,
      computedReplications,
      initReplicationsPage,
      setReplicationPaused,
      deleteReplication,
      selectedReplicationsCount,
      isAnyReplicationsSelected,
      selectedReplications,
      isSelectedProcessing,
      resumeReplications,
      pauseReplications,
      deleteReplications,
      isFiltered,
      clearFilters,
      $reset,
    };
  },
);
