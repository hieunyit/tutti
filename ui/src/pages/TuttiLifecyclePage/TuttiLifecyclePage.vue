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
import {
  computed,
  onMounted,
  ref,
  h,
  type VNode,
} from 'vue';
import {
  NButton,
  NDataTable,
  NModal,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NSelect,
  NSwitch,
  NTag,
  NSpace,
  NDrawer,
  NDrawerContent,
  NPopconfirm,
  NAlert,
  NSpin,
  type DataTableColumns,
} from 'naive-ui';
import { useNotification } from '@clyso/clyso-ui-kit';
import { storeToRefs } from 'pinia';
import { PageShell } from '@/components/kit';
import WriteGate from '@/components/auth/WriteGate.vue';
import { useAuthStore } from '@/stores/authStore';
import { LifecycleService } from '@/services/LifecycleService';
import { RuntimeStorageService } from '@/services/RuntimeStorageService';
import type {
  LifecycleRule,
  LifecycleDryRunResponse,
  LifecycleAuditLogEntry,
} from '@/utils/types/lifecycle';
import { formatDateTime } from '@/utils/helpers/DateTimeHelper';

const { createNotification } = useNotification();
const authStore = useAuthStore();
const { canWrite } = storeToRefs(authStore);

function notifySuccess(content: string) {
  createNotification({ type: 'success', title: 'Success', content });
}

function notifyWarning(content: string) {
  createNotification({ type: 'warning', title: 'Warning', content });
}

function notifyError(err: unknown, fallback: string) {
  let content = fallback;

  if (err && typeof err === 'object') {
    const axiosErr = err as { response?: { data?: { message?: string } }; message?: string };

    if (axiosErr.response?.data?.message) {
      content = axiosErr.response.data.message;
    } else if (axiosErr.message) {
      content = axiosErr.message;
    }
  } else if (err instanceof Error) {
    content = err.message;
  }

  createNotification({ type: 'error', title: 'Error', content });
}

const loading = ref(false);
const loadError = ref(false);
const saving = ref(false);
const rules = ref<LifecycleRule[]>([]);
const storages = ref<string[]>([]);
const updatingRuleIds = ref<Set<string>>(new Set());

// Modal State
const showEditModal = ref(false);
const isEditing = ref(false);
const formModel = ref({
  id: '',
  name: '',
  storageName: '',
  bucket: '',
  prefix: '',
  expirationDays: 30,
  schedule: '0 2 * * *',
  enabled: true,
});

// Dry Run State
const dryRunLoading = ref(false);
const showDryRunModal = ref(false);
const dryRunResult = ref<LifecycleDryRunResponse | null>(null);

// Audit Drawer State
const showAuditDrawer = ref(false);
const auditLoading = ref(false);
const auditLogs = ref<LifecycleAuditLogEntry[]>([]);
const currentRule = ref<LifecycleRule | null>(null);
let auditRequestId = 0;

async function loadRules() {
  loading.value = true;
  loadError.value = false;

  try {
    rules.value = await LifecycleService.listRules();
  } catch (err: unknown) {
    loadError.value = true;
    notifyError(err, 'Failed to load lifecycle rules');
  } finally {
    loading.value = false;
  }
}

async function loadStorages() {
  try {
    const list = await RuntimeStorageService.listSummary();

    storages.value = list.storages?.map((s) => s.name) || [];
  } catch {
    // ignore
  }
}

onMounted(() => {
  loadRules();
  loadStorages();
});

const storageOptions = computed(() =>
  storages.value.map((s) => ({ label: s, value: s })),
);

const pageState = computed<'loading' | 'empty' | 'error' | 'ready'>(() => {
  if (loading.value && rules.value.length === 0) return 'loading';

  if (loadError.value && rules.value.length === 0) return 'error';

  return 'ready';
});

function openCreateModal() {
  if (storages.value.length === 0) {
    loadStorages();
  }

  isEditing.value = false;
  formModel.value = {
    id: '',
    name: '',
    storageName: storages.value[0] || '',
    bucket: '',
    prefix: '',
    expirationDays: 30,
    schedule: '0 2 * * *',
    enabled: true,
  };
  showEditModal.value = true;
}

function openEditModal(rule: LifecycleRule) {
  isEditing.value = true;
  formModel.value = {
    id: rule.id,
    name: rule.name,
    storageName: rule.storageName,
    bucket: rule.bucket,
    prefix: rule.prefix || '',
    expirationDays: rule.expirationDays,
    schedule: rule.schedule || '0 2 * * *',
    enabled: rule.enabled,
  };
  showEditModal.value = true;
}

async function handleSaveRule() {
  const name = formModel.value.name?.trim() || '';
  const storage = formModel.value.storageName?.trim() || '';
  const bucket = formModel.value.bucket?.trim() || '';
  const days = Number(formModel.value.expirationDays);

  if (!name || !storage || !bucket) {
    notifyWarning('Please fill in required fields (Name, Storage, Bucket)');

    return;
  }

  if (isNaN(days) || days < 1 || days > 36500) {
    notifyWarning('Expiration TTL must be between 1 and 36,500 days');

    return;
  }

  saving.value = true;

  try {
    if (isEditing.value) {
      await LifecycleService.updateRule({
        id: formModel.value.id,
        name,
        prefix: formModel.value.prefix ?? '',
        expirationDays: days,
        schedule: formModel.value.schedule?.trim() || '0 2 * * *',
        enabled: formModel.value.enabled,
      });
      notifySuccess('Lifecycle rule updated');
    } else {
      await LifecycleService.createRule({
        name,
        storageName: storage,
        bucket,
        prefix: formModel.value.prefix ?? '',
        expirationDays: days,
        schedule: formModel.value.schedule?.trim() || '0 2 * * *',
        enabled: formModel.value.enabled,
      });
      notifySuccess('Lifecycle rule created');
    }

    showEditModal.value = false;
    await loadRules();
  } catch (err: unknown) {
    notifyError(err, 'Failed to save lifecycle rule');
  } finally {
    saving.value = false;
  }
}

async function handleDeleteRule(id: string) {
  try {
    await LifecycleService.deleteRule(id);
    notifySuccess('Rule deleted');
    await loadRules();
  } catch (err: unknown) {
    notifyError(err, 'Failed to delete rule');
  }
}

async function handleToggleEnabled(rule: LifecycleRule, val: boolean) {
  if (updatingRuleIds.value.has(rule.id)) return;

  updatingRuleIds.value.add(rule.id);

  try {
    if (val) {
      await LifecycleService.enableRule(rule.id);
    } else {
      await LifecycleService.disableRule(rule.id);
    }

    rule.enabled = val;
    notifySuccess(val ? 'Rule enabled' : 'Rule disabled');
  } catch (err: unknown) {
    notifyError(err, 'Failed to update rule state');
  } finally {
    updatingRuleIds.value.delete(rule.id);
  }
}

async function runDryRun(storageName: string, bucket: string, prefix: string, expirationDays: number) {
  dryRunLoading.value = true;
  dryRunResult.value = null;
  showDryRunModal.value = true;

  try {
    dryRunResult.value = await LifecycleService.dryRun({
      storageName,
      bucket,
      prefix,
      expirationDays,
      sampleSize: 10,
    });
  } catch (err: unknown) {
    notifyError(err, 'Dry run failed');
  } finally {
    dryRunLoading.value = false;
  }
}

async function openAuditLogs(rule: LifecycleRule) {
  const requestId = ++auditRequestId;
  const isCurrentRequest = () => requestId === auditRequestId && showAuditDrawer.value;

  currentRule.value = rule;
  auditLogs.value = [];
  showAuditDrawer.value = true;
  auditLoading.value = true;

  try {
    const res = await LifecycleService.listAuditLogs(rule.id, 100, 0);

    if (isCurrentRequest()) auditLogs.value = res.entries || [];
  } catch (err: unknown) {
    if (isCurrentRequest()) notifyError(err, 'Failed to load audit logs');
  } finally {
    if (isCurrentRequest()) auditLoading.value = false;
  }
}

function formatBytes(bytes: string | number | undefined): string {
  if (bytes === undefined || bytes === null) return '0 B';

  const num = typeof bytes === 'string' ? Number(bytes) : bytes;

  if (isNaN(num) || num <= 0) return '0 B';

  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB'];
  const i = Math.min(Math.floor(Math.log(num) / Math.log(1024)), units.length - 1);

  return `${(num / Math.pow(1024, i)).toFixed(2)} ${units[i]}`;
}

function formatRuleDate(val: string | undefined): string {
  if (!val) return 'Never';

  const str = formatDateTime(val);

  if (!str || str === '–' || str.startsWith('0001') || str.includes('1970')) {
    return 'Never';
  }

  return str;
}

const columns: DataTableColumns<LifecycleRule> = [
  {
    title: 'Name',
    key: 'name',
    render: (row) => h('strong', row.name),
  },
  {
    title: 'Target',
    key: 'target',
    render: (row) =>
      h('div', [
        h('span', { class: 'text-gray-500 font-mono text-xs' }, row.storageName + ' / '),
        h('span', { class: 'font-semibold text-xs' }, row.bucket),
        row.prefix ? h('span', { class: 'text-gray-400 text-xs' }, ` (${row.prefix}*)`) : null,
      ]),
  },
  {
    title: 'TTL',
    key: 'expirationDays',
    render: (row) => h(NTag, { type: 'warning', size: 'small' }, { default: () => `${row.expirationDays} days` }),
  },
  {
    title: 'Schedule',
    key: 'schedule',
    render: (row) => h('code', { class: 'text-xs' }, row.schedule || '0 2 * * *'),
  },
  {
    title: 'Status',
    key: 'enabled',
    render: (row) =>
      h(NSwitch, {
        disabled: !canWrite.value || updatingRuleIds.value.has(row.id),
        loading: updatingRuleIds.value.has(row.id),
        value: row.enabled,
        onUpdateValue: (val: boolean) => handleToggleEnabled(row, val),
      }),
  },
  {
    title: 'Last Run',
    key: 'lastRunAt',
    render: (row) => {
      if (!row.lastRunAt) return h('span', { class: 'text-gray-400' }, 'Never');

      const stats = row.lastRunStats;
      const statsText = stats ? ` (${stats.deletedCount} deleted, ${formatBytes(stats.bytesFreed)})` : '';

      return h('div', { class: 'text-xs' }, [
        h('div', formatRuleDate(row.lastRunAt)),
        h('div', { class: 'text-gray-500 text-2xs' }, statsText),
      ]);
    },
  },
  {
    title: 'Actions',
    key: 'actions',
    render: (row) => {
      const btns: VNode[] = [];

      if (canWrite.value) {
        btns.push(
          h(
            NButton,
            {
              size: 'tiny',
              quaternary: true,
              onClick: () => runDryRun(row.storageName, row.bucket, row.prefix, row.expirationDays),
            },
            { default: () => 'Dry Run' },
          ),
        );
      }

      btns.push(
        h(
          NButton,
          {
            size: 'tiny',
            quaternary: true,
            onClick: () => openAuditLogs(row),
          },
          { default: () => 'Logs' },
        ),
      );

      if (canWrite.value) {
        btns.push(
          h(
            NButton,
            {
              size: 'tiny',
              quaternary: true,
              onClick: () => openEditModal(row),
            },
            { default: () => 'Edit' },
          ),
          h(
            NPopconfirm,
            {
              onPositiveClick: () => handleDeleteRule(row.id),
            },
            {
              trigger: () =>
                h(NButton, { size: 'tiny', quaternary: true, type: 'error' }, { default: () => 'Delete' }),
              default: () => 'Are you sure you want to delete this lifecycle rule?',
            },
          ),
        );
      }

      return h(NSpace, { size: 'small' }, () => btns);
    },
  },
];
</script>

<template>
  <PageShell
    title="Lifecycle Rules"
    description="Automate object expiration and cleanup based on TTL policies"
    :state="pageState"
  >
    <template #actions>
      <WriteGate>
        <NButton
          type="primary"
          @click="openCreateModal"
        >
          Add Lifecycle Rule
        </NButton>
      </WriteGate>
    </template>

    <div class="lifecycle-page">
      <NAlert type="info">
        Automatic expiration requires enabled bucket versioning. Keys with older live versions
        are retained to prevent older data from becoming visible after expiration.
      </NAlert>
      <NDataTable
        :loading="loading"
        :columns="columns"
        :data="rules"
        :row-key="(row: LifecycleRule) => row.id"
      />
    </div>

    <!-- Create / Edit Modal -->
    <NModal
      v-model:show="showEditModal"
      preset="card"
      :title="isEditing ? 'Edit Lifecycle Rule' : 'New Lifecycle Rule'"
      style="max-width: 600px"
    >
      <NForm label-placement="top">
        <NFormItem
          label="Rule Name"
          required
        >
          <NInput
            v-model:value="formModel.name"
            placeholder="e.g. cleanup-logs"
          />
        </NFormItem>

        <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 16px">
          <NFormItem
            label="Storage"
            required
          >
            <NSelect
              v-model:value="formModel.storageName"
              :options="storageOptions"
              :disabled="isEditing"
              placeholder="Select Storage"
            />
          </NFormItem>
          <NFormItem
            label="Bucket"
            required
          >
            <NInput
              v-model:value="formModel.bucket"
              :disabled="isEditing"
              placeholder="Bucket name"
            />
          </NFormItem>
        </div>

        <NFormItem label="Object Prefix (Optional)">
          <NInput
            v-model:value="formModel.prefix"
            placeholder="e.g. temp/ or logs/"
          />
        </NFormItem>

        <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 16px">
          <NFormItem
            label="Expiration TTL (Days)"
            required
          >
            <NInputNumber
              v-model:value="formModel.expirationDays"
              :min="1"
              style="width: 100%"
            />
          </NFormItem>
          <NFormItem label="Cron Schedule">
            <NInput
              v-model:value="formModel.schedule"
              placeholder="0 2 * * *"
            />
          </NFormItem>
        </div>

        <NFormItem label="Enabled">
          <NSwitch v-model:value="formModel.enabled" />
        </NFormItem>
      </NForm>

      <template #footer>
        <NSpace
          justify="space-between"
          align="center"
        >
          <NButton
            v-if="canWrite"
            quaternary
            :loading="dryRunLoading"
            :disabled="!formModel.storageName || !formModel.bucket || dryRunLoading"
            @click="runDryRun(formModel.storageName, formModel.bucket, formModel.prefix, formModel.expirationDays)"
          >
            Test Dry Run
          </NButton>
          <NSpace>
            <NButton @click="showEditModal = false">
              Cancel
            </NButton>
            <NButton
              type="primary"
              :loading="saving"
              @click="handleSaveRule"
            >
              Save
            </NButton>
          </NSpace>
        </NSpace>
      </template>
    </NModal>

    <!-- Dry Run Modal -->
    <NModal
      v-model:show="showDryRunModal"
      preset="card"
      title="Lifecycle Dry Run Simulation"
      style="max-width: 640px"
    >
      <div
        v-if="dryRunLoading"
        style="text-align: center; padding: 32px"
      >
        <NSpin size="large" />
        <div style="margin-top: 12px; color: #888888">
          Scanning bucket objects...
        </div>
      </div>
      <div v-else-if="dryRunResult">
        <NAlert
          type="info"
          title="Simulation Summary"
          style="margin-bottom: 16px"
        >
          <div>Estimated Objects to Delete: <strong>{{ dryRunResult.estimatedDeletedCount }}</strong></div>
          <div>Estimated Space Reclaimed: <strong>{{ formatBytes(dryRunResult.estimatedBytesFreed) }}</strong></div>
        </NAlert>

        <div v-if="dryRunResult.sampleObjects && dryRunResult.sampleObjects.length > 0">
          <h4 style="margin-bottom: 8px">
            Sample Expired Objects (up to 10):
          </h4>
          <ul style="max-height: 200px; overflow-y: auto; background: var(--n-color); padding: 8px 16px; border-radius: 4px; font-family: monospace; font-size: 12px">
            <li
              v-for="key in dryRunResult.sampleObjects"
              :key="key"
            >
              {{ key }}
            </li>
          </ul>
        </div>
        <div
          v-else
          class="text-gray-400"
        >
          No matching expired objects found.
        </div>
      </div>
      <template #footer>
        <NButton @click="showDryRunModal = false">
          Close
        </NButton>
      </template>
    </NModal>

    <!-- Audit Logs Drawer -->
    <NDrawer
      v-model:show="showAuditDrawer"
      :width="680"
      placement="right"
    >
      <NDrawerContent :title="`Deletion Audit: ${currentRule?.name || ''}`">
        <div
          v-if="auditLoading"
          style="text-align: center; padding: 32px"
        >
          <NSpin />
        </div>
        <div
          v-else-if="auditLogs.length === 0"
          style="text-align: center; padding: 32px; color: #888888"
        >
          No deletion records found for this rule.
        </div>
        <div v-else>
          <div style="margin-bottom: 12px; color: #888888; font-size: 12px">
            Showing {{ auditLogs.length }} recent deletions
          </div>
          <table style="width: 100%; border-collapse: collapse; font-size: 12px">
            <thead>
              <tr style="border-bottom: 1px solid var(--n-border-color); text-align: left">
                <th style="padding: 8px">
                  Object Key
                </th>
                <th style="padding: 8px">
                  Size
                </th>
                <th style="padding: 8px">
                  Deleted At
                </th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="log in auditLogs"
                :key="log.id"
                style="border-bottom: 1px solid var(--n-border-color)"
              >
                <td style="padding: 8px; font-family: monospace; word-break: break-all">
                  {{ log.objectKey }}
                </td>
                <td style="padding: 8px; white-space: nowrap">
                  {{ formatBytes(log.sizeBytes) }}
                </td>
                <td style="padding: 8px; white-space: nowrap">
                  {{ formatDateTime(log.deletedAt) }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </NDrawerContent>
    </NDrawer>
  </PageShell>
</template>

<style scoped>
.lifecycle-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
</style>
