<!--
  - Copyright © 2026 hieunyit
  -
  -  Licensed under the GNU Affero General Public License, Version 3.0 (the "License");
  -  you may not use this file except in compliance with the License.
  -  You may obtain a copy of the License at
  -
  -  https://www.gnu.org/licenses/agpl-3.0.html
  -
  -  Unless required by applicable law or agreed to in writing, software
  -  distributed under the License is distributed on an "AS IS" BASIS,
  -  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  -  See the License for the specific language governing permissions and
  -  limitations under the License.
  -->

<script setup lang="ts">
  /**
   * Key table with every access key for one account (user detail) or all
   * accounts (flat access keys page).
   * Static keys come from the config and are read-only; managed keys can be
   * rotated, disabled/enabled and deleted.
   */
  import { CButton } from '@clyso/clyso-ui-kit';
  import { computed, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { RouterLink } from 'vue-router';
  import i18nAccounts from './i18nAccounts';
  import CopyButton from '@/components/chorus/common/CopyButton/CopyButton.vue';
  import ConfirmDialog from '@/components/tutti/common/ConfirmDialog.vue';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import StatusTag from '@/components/ui/StatusTag.vue';
  import { EMPTY_VALUE } from '@/components/ui/status';
  import { useTuttiAccessPolicyStore } from '@/stores/tuttiAccessPolicyStore';
  import { useChorusNotification } from '@/utils/composables/useChorusNotification';
  import { formatDateTime, formatTime } from '@/utils/helpers/DateTimeHelper';
  import {
    ProxyAccountHelper,
    type ProxyAccountKeyRow,
  } from '@/utils/helpers/ProxyAccountHelper';
  import type { TuttiProxyAccount } from '@/utils/types/tutti';
  import { RouteName } from '@/utils/types/router';

  const props = defineProps<{
    account?: TuttiProxyAccount;
    accounts?: TuttiProxyAccount[];
  }>();

  const emit = defineEmits<{
    (e: 'rotate', accessKeyId: string): void;
  }>();

  const { t } = useI18n({ messages: i18nAccounts });
  const store = useTuttiAccessPolicyStore();
  const { createNotification } = useChorusNotification();

  const rows = computed(() =>
    (props.accounts ?? (props.account ? [props.account] : [])).flatMap(
      (account) =>
        ProxyAccountHelper.getKeyRows(account).map((row) => ({
          ...row,
          user: account.user,
        })),
    ),
  );
  const deleting = ref<ProxyAccountKeyRow | null>(null);
  const isDeleting = ref(false);

  function reportError(error: unknown) {
    createNotification({
      type: 'error',
      title: ProxyAccountHelper.isNotConfigured(error)
        ? t('notConfigured')
        : (ProxyAccountHelper.getMessage(error) ?? t('actionUnknownError')),
      duration: 8000,
    });
  }

  async function toggleStatus(row: ProxyAccountKeyRow) {
    if (!row.dynamicKey) return;

    try {
      await store.setKeyStatus(
        row.accessKeyId,
        ProxyAccountHelper.isDisabled(row.dynamicKey) ? 'active' : 'disabled',
      );
    } catch (error: unknown) {
      reportError(error);
    }
  }

  async function confirmDelete() {
    if (!deleting.value) return;

    isDeleting.value = true;

    try {
      await store.deleteKey(deleting.value.accessKeyId);
      deleting.value = null;
    } catch (error: unknown) {
      reportError(error);
    } finally {
      isDeleting.value = false;
    }
  }

  function sourceLabel(source: string): string {
    if (source === 'dynamic') return t('sourceManaged');

    if (source.startsWith('storage:')) {
      return t('sourceConfigStorage', {
        name: source.slice('storage:'.length),
      });
    }

    return t('sourceConfig');
  }

  function dateCell(value?: string): string {
    return ProxyAccountHelper.hasTime(value)
      ? formatDateTime(value)
      : EMPTY_VALUE;
  }

  /** "old secret valid until HH:MM:SS", only while it is still valid */
  function oldSecretNote(row: ProxyAccountKeyRow): string | null {
    const expires = row.dynamicKey?.previousSecretExpiresAt;

    if (!ProxyAccountHelper.hasTime(expires)) return null;

    return Date.parse(expires as string) > Date.now()
      ? t('oldSecretValidUntil', { time: formatTime(expires) })
      : null;
  }
</script>

<template>
  <div :class="['keys-table', { 'keys-table--flat': !!accounts }]">
    <p
      v-if="rows.length === 0"
      class="keys-table__none"
    >
      {{ t('noKeys') }}
    </p>
    <div
      v-else
      class="tt-table-wrap keys-table__wrap"
    >
      <table class="tt-table">
        <thead>
          <tr>
            <th
              v-if="accounts"
              scope="col"
            >
              {{ t('columnUser') }}
            </th>
            <th scope="col">{{ t('columnAccessKeyId') }}</th>
            <th scope="col">{{ t('columnSource') }}</th>
            <th scope="col">{{ t('columnStatus') }}</th>
            <th scope="col">{{ t('columnDescription') }}</th>
            <th scope="col">{{ t('columnCreated') }}</th>
            <th scope="col">{{ t('columnRotated') }}</th>
            <th scope="col">
              <span class="tt-sr-only">{{ t('columnActions') }}</span>
            </th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="row in rows"
            :key="`${row.user}:${row.accessKeyId}`"
          >
            <td v-if="accounts">
              <RouterLink
                :to="{
                  name: RouteName.TUTTI_USER_DETAIL,
                  params: { name: row.user },
                }"
                >{{ row.user }}</RouterLink
              >
            </td>
            <td :data-label="accounts ? t('columnAccessKeyId') : undefined">
              <span class="keys-table__id">
                <code>{{ row.accessKeyId }}</code>
                <CopyButton :text="row.accessKeyId" />
              </span>
            </td>
            <td :data-label="accounts ? t('columnSource') : undefined">
              <span class="keys-table__source">{{
                sourceLabel(row.source)
              }}</span>
            </td>
            <td :data-label="accounts ? t('columnStatus') : undefined">
              <StatusTag
                :variant="
                  row.dynamicKey &&
                  ProxyAccountHelper.isDisabled(row.dynamicKey)
                    ? 'neutral'
                    : 'success'
                "
                :label="
                  row.dynamicKey &&
                  ProxyAccountHelper.isDisabled(row.dynamicKey)
                    ? t('statusDisabled')
                    : t('statusActive')
                "
              />
            </td>
            <td :data-label="accounts ? t('columnDescription') : undefined">
              {{ row.dynamicKey?.description || EMPTY_VALUE }}
            </td>
            <td :data-label="accounts ? t('columnCreated') : undefined">
              {{ dateCell(row.dynamicKey?.createdAt) }}
            </td>
            <td :data-label="accounts ? t('columnRotated') : undefined">
              <span class="keys-table__stack">
                <span>{{ dateCell(row.dynamicKey?.rotatedAt) }}</span>
                <span
                  v-if="oldSecretNote(row)"
                  class="keys-table__warn"
                  >{{ oldSecretNote(row) }}</span
                >
              </span>
            </td>
            <td
              class="keys-table__actions"
              :data-label="accounts ? t('columnActions') : undefined"
            >
              <span
                v-if="!row.dynamicKey"
                class="keys-table__muted"
                >{{ t('managedInConfig') }}</span
              >
              <WriteGate v-else>
                <span class="keys-table__buttons">
                  <CButton
                    secondary
                    size="small"
                    @click="emit('rotate', row.accessKeyId)"
                  >
                    {{ t('rotate') }}
                  </CButton>
                  <CButton
                    secondary
                    size="small"
                    @click="toggleStatus(row)"
                  >
                    {{
                      ProxyAccountHelper.isDisabled(row.dynamicKey)
                        ? t('enableKey')
                        : t('disableKey')
                    }}
                  </CButton>
                  <CButton
                    type="error"
                    ghost
                    size="small"
                    @click="deleting = row"
                  >
                    {{ t('deleteKey') }}
                  </CButton>
                </span>
              </WriteGate>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <ConfirmDialog
      :show="deleting !== null"
      :title="t('deleteKeyDialogTitle')"
      :text="t('deleteKeyConfirm', { key: deleting?.accessKeyId ?? '' })"
      :confirm-label="t('deleteKeyAction')"
      :cancel-label="t('cancel')"
      :loading="isDeleting"
      danger
      @update:show="deleting = null"
      @confirm="confirmDelete"
    />
  </div>
</template>

<style lang="scss" scoped>
  .keys-table {
    background-color: var(--tt-surface);
    border: 1px solid var(--tt-border);
    border-radius: 10px;
    overflow: hidden;

    :deep(.tt-table th),
    :deep(.tt-table td) {
      padding-inline: 10px;
    }

    &__none {
      margin: 0;
      padding: 12px 16px;
      font-size: 13px;
      color: var(--tt-text-3);
    }

    &__id {
      display: inline-flex;
      align-items: center;
      gap: 6px;

      code {
        font-family: var(--tt-mono);
        font-size: 12px;
      }
    }

    &__source {
      padding: 1px 8px;
      font-size: 12px;
      color: var(--tt-text-2);
      border: 1px solid var(--tt-border-strong);
      border-radius: var(--tt-radius-tag);
    }

    &__stack {
      display: flex;
      flex-direction: column;
    }

    &__warn {
      font-size: 12px;
      color: var(--tt-warning);
    }

    &__muted {
      font-size: 12px;
      color: var(--tt-text-3);
    }

    &__actions {
      text-align: right;
    }

    &__buttons {
      display: inline-flex;
      gap: 6px;
    }

    @media (max-width: 600px) {
      &--flat .tt-table,
      &--flat .tt-table tbody,
      &--flat .tt-table tr,
      &--flat .tt-table td {
        display: block;
        width: 100%;
        box-sizing: border-box;
      }

      &--flat .tt-table thead {
        display: none;
      }

      &--flat .tt-table tr {
        padding: 8px 12px;
        border-bottom: 1px solid var(--tt-border);
      }

      &--flat .tt-table td {
        display: flex;
        justify-content: space-between;
        align-items: flex-start;
        gap: 12px;
        padding: 6px 0;
        white-space: normal;
        overflow-wrap: anywhere;
        border: 0;
        text-align: right;
      }

      &--flat .tt-table td::before {
        content: attr(data-label);
        flex: 0 0 35%;
        color: var(--tt-text-3);
        font-size: 12px;
        text-align: left;
      }

      &--flat .keys-table__buttons {
        flex-wrap: wrap;
        justify-content: flex-end;
      }
    }
  }
</style>
