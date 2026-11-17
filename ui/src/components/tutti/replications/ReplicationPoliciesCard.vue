<!--
  - Copyright © 2026 hieunyit
  -
  -  Licensed under the GNU Affero General Public License, Version 3.0 (the "License");
  -  you may not use this file except in compliance with the License.
  -  You may obtain a copy of the License at
  -
  -  https://www.gnu.org/licenses/agpl-3.0.html
  -->
<script setup lang="ts">
import { ref, watch, h } from 'vue';
import {
  NButton,
  NSelect,
  NInputNumber,
  NDynamicTags,
  NSpace,
  NModal,
  NDataTable,
  NTag,
  NSpin,
  type DataTableColumns,
} from 'naive-ui';
import { useNotification } from '@clyso/clyso-ui-kit';
import { storeToRefs } from 'pinia';
import { SectionCard } from '@/components/kit';
import WriteGate from '@/components/auth/WriteGate.vue';
import { useAuthStore } from '@/stores/authStore';
import { TuttiCoreService } from '@/services/TuttiCoreService';
import {
  DeletePropagationMode,
  type TuttiReplication,
  type DeleteEvent,
} from '@/utils/types/tuttiCore';
import { formatDateTime } from '@/utils/helpers/DateTimeHelper';

const props = defineProps<{
  replication: TuttiReplication;
  reload: () => Promise<void>;
}>();

const { createNotification } = useNotification();
const { canWrite } = storeToRefs(useAuthStore());

function notifySuccess(content: string) {
  createNotification({ type: 'success', title: 'Success', content });
}

function notifyError(err: unknown, fallback: string) {
  let content = fallback;

  if (err && typeof err === 'object') {
    const axiosErr = err as {
      response?: { data?: { message?: string; error?: string } };
      message?: string;
    };

    if (axiosErr.response?.data?.message) {
      content = axiosErr.response.data.message;
    } else if (axiosErr.response?.data?.error) {
      content = axiosErr.response.data.error;
    } else if (axiosErr.message) {
      content = axiosErr.message;
    }
  } else if (err instanceof Error) {
    content = err.message;
  }

  createNotification({ type: 'error', title: 'Error', content });
}

// Delete Propagation State
const deleteMode = ref<DeletePropagationMode>(DeletePropagationMode.IGNORE);
const deleteIncludePrefixes = ref<string[]>([]);
const deleteExcludePrefixes = ref<string[]>([]);
const deleteDelaySeconds = ref<number>(0);
const savingDeletePolicy = ref(false);
const isDeleteDirty = ref(false);

// Filter State
const includePatterns = ref<string[]>([]);
const excludePatterns = ref<string[]>([]);
const savingFilters = ref(false);
const isFiltersDirty = ref(false);

function getCanonicalReplicationId(): string {
  const id = props.replication?.id;

  if (!id) return '';

  if (id.fromBucket && id.toBucket) {
    return [id.user || '', id.fromStorage || '', id.toStorage || '', id.fromBucket, id.toBucket].join(':');
  }

  return [id.user || '', id.fromStorage || '', id.toStorage || ''].join(':');
}

// Delete Events Modal
const showEventsModal = ref(false);
const eventsLoading = ref(false);
const deleteEvents = ref<DeleteEvent[]>([]);
const retryingDeletes = ref(false);

function initDeletePolicyFromProps(rep: TuttiReplication) {
  if (!rep) return;

  const dp = rep.deletePropagation;
  let rawMode: string | number | undefined;
  let prefixes: string[] = [];
  let exPrefixes: string[] = [];
  let delaySec = 0;

  if (dp && typeof dp === 'object') {
    rawMode = dp.mode;
    prefixes = dp.includePrefixes || [];
    exPrefixes = dp.excludePrefixes || [];

    if (typeof dp.delayBeforeDelete === 'string') {
      delaySec = parseInt(dp.delayBeforeDelete, 10) || 0;
    } else if (dp.delayBeforeDelete && typeof dp.delayBeforeDelete === 'object') {
      delaySec = parseInt(dp.delayBeforeDelete.seconds || '0', 10) || 0;
    }
  } else if (typeof dp === 'string') {
    rawMode = dp;
    prefixes = rep.deleteIncludePrefixes || [];
    exPrefixes = rep.deleteExcludePrefixes || [];
    delaySec = rep.deleteDelaySeconds || 0;
  }

  if (
    rawMode === 'sync' ||
    rawMode === 'DELETE_PROPAGATION_SYNC' ||
    rawMode === DeletePropagationMode.SYNC
  ) {
    deleteMode.value = DeletePropagationMode.SYNC;
  } else if (
    rawMode === 'marker-only' ||
    rawMode === 'DELETE_PROPAGATION_MARKER_ONLY' ||
    rawMode === DeletePropagationMode.MARKER_ONLY
  ) {
    deleteMode.value = DeletePropagationMode.MARKER_ONLY;
  } else {
    deleteMode.value = DeletePropagationMode.IGNORE;
  }

  deleteIncludePrefixes.value = [...prefixes];
  deleteExcludePrefixes.value = [...exPrefixes];
  deleteDelaySeconds.value = delaySec;
}

function initFiltersFromProps(rep: TuttiReplication) {
  if (!rep) return;

  includePatterns.value = [...(rep.includePatterns || [])];
  excludePatterns.value = [...(rep.excludePatterns || [])];
}

function revertDeletePolicy() {
  isDeleteDirty.value = false;
  initDeletePolicyFromProps(props.replication);
}

function revertFilters() {
  isFiltersDirty.value = false;
  initFiltersFromProps(props.replication);
}

watch(
  () => props.replication,
  (rep) => {
    if (!rep) return;

    if (!isDeleteDirty.value) {
      initDeletePolicyFromProps(rep);
    }

    if (!isFiltersDirty.value) {
      initFiltersFromProps(rep);
    }
  },
  { immediate: true },
);

const modeOptions = [
  { label: 'Ignore (Do not sync deletes)', value: DeletePropagationMode.IGNORE },
  { label: 'Sync (Execute hard deletes at destination)', value: DeletePropagationMode.SYNC },
  { label: 'Marker-Only (Create delete markers at destination)', value: DeletePropagationMode.MARKER_ONLY },
];

async function handleSaveDeletePropagation() {
  savingDeletePolicy.value = true;

  try {
    const delaySec = Math.max(0, Number(deleteDelaySeconds.value) || 0);

    await TuttiCoreService.updateDeletePropagation({
      id: props.replication.id,
      deletePropagation: {
        mode: deleteMode.value,
        includePrefixes: deleteIncludePrefixes.value,
        excludePrefixes: deleteExcludePrefixes.value,
        delayBeforeDelete: `${delaySec}s`,
      },
    });
    isDeleteDirty.value = false;
    notifySuccess('Delete propagation policy updated');
    await props.reload();
  } catch (err: unknown) {
    notifyError(err, 'Failed to update delete propagation');
  } finally {
    savingDeletePolicy.value = false;
  }
}

async function handleSaveFilters() {
  savingFilters.value = true;

  try {
    await TuttiCoreService.updateReplicationFilters({
      id: props.replication.id,
      includePatterns: includePatterns.value,
      excludePatterns: excludePatterns.value,
    });
    isFiltersDirty.value = false;
    notifySuccess('Replication pattern filters updated');
    await props.reload();
  } catch (err: unknown) {
    notifyError(err, 'Failed to update pattern filters');
  } finally {
    savingFilters.value = false;
  }
}

async function openDeleteEvents() {
  showEventsModal.value = true;
  eventsLoading.value = true;

  try {
    const res = await TuttiCoreService.listDeleteEvents({
      replicationId: getCanonicalReplicationId(),
      limit: 50,
      offset: 0,
    });

    deleteEvents.value = res.events || [];
  } catch (err: unknown) {
    notifyError(err, 'Failed to load delete events');
  } finally {
    eventsLoading.value = false;
  }
}

async function handleRetryFailedDeletes() {
  retryingDeletes.value = true;

  try {
    await TuttiCoreService.retryFailedDeletes({
      replicationId: getCanonicalReplicationId(),
    });
    notifySuccess('Failed delete events re-enqueued for retry');
    await openDeleteEvents();
  } catch (err: unknown) {
    notifyError(err, 'Failed to retry delete events');
  } finally {
    retryingDeletes.value = false;
  }
}

const eventColumns: DataTableColumns<DeleteEvent> = [
  {
    title: 'Object Key',
    key: 'objectKey',
    render: (row) => h('code', { class: 'text-xs' }, row.objectKey),
  },
  {
    title: 'Type',
    key: 'deletionType',
    render: (row) => h(NTag, { size: 'small', type: 'info' }, { default: () => row.deletionType }),
  },
  {
    title: 'Status',
    key: 'status',
    render: (row) => {
      const type = row.status === 'success' ? 'success' : row.status === 'failed' ? 'error' : 'warning';

      return h(NTag, { size: 'small', type }, { default: () => row.status });
    },
  },
  {
    title: 'Retries',
    key: 'retryCount',
    render: (row) => row.retryCount,
  },
  {
    title: 'Error',
    key: 'lastError',
    render: (row) => (row.lastError ? h('span', { class: 'text-red-500 text-xs' }, row.lastError) : '-'),
  },
  {
    title: 'Created At',
    key: 'createdAt',
    render: (row) => formatDateTime(row.createdAt),
  },
];
</script>

<template>
  <SectionCard title="Delete Propagation & Advanced Filtering">
    <div class="policies-container">
      <!-- Delete Propagation Section -->
      <div class="policy-block">
        <h4 class="font-semibold text-sm mb-2">Delete Propagation Policy</h4>
        <div class="flex flex-col gap-3">
          <div>
            <label class="text-xs text-gray-500 mb-1 block">Mode</label>
            <NSelect
              v-model:value="deleteMode"
              :options="modeOptions"
              :disabled="!canWrite || savingDeletePolicy"
              @update:value="isDeleteDirty = true"
            />
          </div>

          <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 12px">
            <div>
              <label class="text-xs text-gray-500 mb-1 block">Include Prefixes (Optional)</label>
              <NDynamicTags
                v-model:value="deleteIncludePrefixes"
                :disabled="!canWrite || savingDeletePolicy"
                @update:value="isDeleteDirty = true"
              />
            </div>
            <div>
              <label class="text-xs text-gray-500 mb-1 block">Exclude Prefixes (Optional)</label>
              <NDynamicTags
                v-model:value="deleteExcludePrefixes"
                :disabled="!canWrite || savingDeletePolicy"
                @update:value="isDeleteDirty = true"
              />
            </div>
          </div>

          <div>
            <label class="text-xs text-gray-500 mb-1 block">Delay Before Delete (Seconds, soft delete)</label>
            <NInputNumber
              v-model:value="deleteDelaySeconds"
              :min="0"
              :disabled="!canWrite || savingDeletePolicy"
              style="width: 200px"
              @update:value="isDeleteDirty = true"
            />
          </div>

          <NSpace
            justify="space-between"
            align="center"
            style="margin-top: 8px"
          >
            <NButton
              quaternary
              size="small"
              @click="openDeleteEvents"
            >
              View Delete Events
            </NButton>
            <NSpace align="center">
              <NButton
                v-if="isDeleteDirty && canWrite"
                quaternary
                size="small"
                :disabled="savingDeletePolicy"
                @click="revertDeletePolicy"
              >
                Revert
              </NButton>
              <WriteGate>
                <NButton
                  type="primary"
                  size="small"
                  :loading="savingDeletePolicy"
                  :disabled="!isDeleteDirty"
                  @click="handleSaveDeletePropagation"
                >
                  Save Delete Policy
                </NButton>
              </WriteGate>
            </NSpace>
          </NSpace>
        </div>
      </div>

      <!-- Advanced Filtering Section -->
      <div class="policy-block">
        <h4 class="font-semibold text-sm mb-2">Advanced Pattern Filtering (Glob)</h4>
        <div class="flex flex-col gap-3">
          <div>
            <label class="text-xs text-gray-500 mb-1 block">
              Include Patterns (e.g. <code>*.jpg</code>, <code>data/**</code>)
            </label>
            <NDynamicTags
              v-model:value="includePatterns"
              :disabled="!canWrite || savingFilters"
              @update:value="isFiltersDirty = true"
            />
          </div>

          <div>
            <label class="text-xs text-gray-500 mb-1 block">
              Exclude Patterns (priority over include, e.g. <code>*.tmp</code>, <code>logs/**</code>)
            </label>
            <NDynamicTags
              v-model:value="excludePatterns"
              :disabled="!canWrite || savingFilters"
              @update:value="isFiltersDirty = true"
            />
          </div>

          <NSpace
            justify="end"
            align="center"
            style="margin-top: 8px"
          >
            <NButton
              v-if="isFiltersDirty && canWrite"
              quaternary
              size="small"
              :disabled="savingFilters"
              @click="revertFilters"
            >
              Revert
            </NButton>
            <WriteGate>
              <NButton
                type="primary"
                size="small"
                :loading="savingFilters"
                :disabled="!isFiltersDirty"
                @click="handleSaveFilters"
              >
                Save Pattern Filters
              </NButton>
            </WriteGate>
          </NSpace>
        </div>
      </div>
    </div>

    <!-- Delete Events Modal -->
    <NModal
      v-model:show="showEventsModal"
      preset="card"
      title="Delete Propagation Events"
      style="max-width: 800px"
    >
      <div
        v-if="eventsLoading"
        style="text-align: center; padding: 32px"
      >
        <NSpin />
      </div>
      <div v-else>
        <NSpace
          justify="space-between"
          align="center"
          style="margin-bottom: 12px"
        >
          <span class="text-xs text-gray-500">Showing recent delete events</span>
          <WriteGate>
            <NButton
              size="small"
              type="warning"
              :loading="retryingDeletes"
              @click="handleRetryFailedDeletes"
            >
              Retry Failed Deletes
            </NButton>
          </WriteGate>
        </NSpace>

        <NDataTable
          :columns="eventColumns"
          :data="deleteEvents"
          :pagination="{ pageSize: 10 }"
          size="small"
        />
      </div>
    </NModal>
  </SectionCard>
</template>

<style scoped>
.policies-container {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 24px;
}
@media (max-width: 768px) {
  .policies-container {
    grid-template-columns: 1fr;
  }
}
.policy-block {
  background: var(--n-color);
  padding: 16px;
  border-radius: 6px;
  border: 1px solid var(--n-border-color);
}
</style>
