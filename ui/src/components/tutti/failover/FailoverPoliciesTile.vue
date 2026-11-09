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
  import { CButton, CIcon } from '@clyso/clyso-ui-kit';
  import { storeToRefs } from 'pinia';
  import { computed, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nFailover from './i18nFailover';
  import FailoverPolicyDrawer from './FailoverPolicyDrawer.vue';
  import ConfirmDialog from '@/components/tutti/common/ConfirmDialog.vue';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import SectionCard from '@/components/ui/SectionCard.vue';
  import StatusTag from '@/components/ui/StatusTag.vue';
  import EmptyState from '@/components/ui/EmptyState.vue';
  import { EMPTY_VALUE } from '@/components/ui/status';
  import { IconName } from '@/utils/types/icon';
  import { useTuttiFailoverStore } from '@/stores/tuttiFailoverStore';
  import { useChorusNotification } from '@/utils/composables/useChorusNotification';
  import {
    FailoverHelper,
    type FailoverTrigger,
  } from '@/utils/helpers/FailoverHelper';
  import type { TuttiFailoverPolicy } from '@/utils/types/tutti';

  const { t } = useI18n({ messages: i18nFailover });
  const store = useTuttiFailoverStore();
  const { policies, proxies, replications } = storeToRefs(store);
  const { createNotification } = useChorusNotification();

  const showDrawer = ref(false);
  const editedPolicy = ref<TuttiFailoverPolicy | null>(null);
  const deleting = ref<TuttiFailoverPolicy | null>(null);
  const isDeleting = ref(false);

  function openAdd() {
    editedPolicy.value = null;
    showDrawer.value = true;
  }

  // the primary "Add policy" action lives in the page header
  defineExpose({ openAdd });

  function openEdit(policy: TuttiFailoverPolicy) {
    editedPolicy.value = policy;
    showDrawer.value = true;
  }

  async function confirmDelete() {
    if (!deleting.value) return;

    isDeleting.value = true;

    const ok = await store.deletePolicy(deleting.value.source);

    isDeleting.value = false;

    if (ok) {
      deleting.value = null;

      return;
    }

    createNotification({
      type: 'error',
      title: t('deleteFailed'),
      duration: 6000,
    });
  }

  const defaultAfter = computed(
    () =>
      proxies.value.find((proxy) => proxy.settings?.failover?.after)?.settings
        ?.failover?.after,
  );

  function seconds(duration?: string): string | null {
    const value = duration
      ? FailoverHelper.parseDurationSeconds(duration)
      : null;

    return value === null ? null : `${value} s`;
  }

  function afterText(policy: TuttiFailoverPolicy): string {
    const own = seconds(policy.after);

    if (own) return own;

    const fallback = seconds(defaultAfter.value);

    return fallback ? t('defaultSuffix', { value: fallback }) : EMPTY_VALUE;
  }

  function lagText(policy: TuttiFailoverPolicy): string {
    if (policy.maxEventLag !== undefined && policy.maxEventLag !== null) {
      return String(policy.maxEventLag);
    }

    return t('defaultSuffix', {
      value: FailoverHelper.effectiveMaxLag(policy, proxies.value),
    });
  }

  const rows = computed(() =>
    policies.value.map((policy) => ({
      policy,
      trigger: FailoverHelper.evaluatePolicy(
        policy,
        proxies.value,
        replications.value,
      ),
    })),
  );

  function triggerText(trigger: FailoverTrigger): string {
    if (trigger.state === 'yes') return t('triggerYes');

    if (trigger.state === 'disabled') return EMPTY_VALUE;

    return trigger.reason === 'proxiesOff'
      ? t('triggerNoProxies')
      : t('triggerNoReplication');
  }
</script>

<template>
  <SectionCard
    class="failover-policies-tile"
    :title="t('policiesTitle')"
    flush
  >
    <EmptyState
      v-if="policies.length === 0"
      :icon="IconName.BASE_REFRESH"
      :title="t('policiesEmptyTitle')"
      :text="t('policiesEmptyText')"
    >
      <WriteGate>
        <CButton
          type="primary"
          @click="openAdd"
        >
          <template #icon>
            <CIcon
              :is-inline="true"
              :name="IconName.BASE_ADD"
            />
          </template>

          {{ t('addPolicy') }}
        </CButton>
      </WriteGate>
    </EmptyState>

    <div
      v-else
      class="tt-table-wrap"
    >
      <table class="tt-table">
        <thead>
          <tr>
            <th scope="col">{{ t('columnSourceStorage') }}</th>
            <th scope="col">{{ t('columnStatus') }}</th>
            <th scope="col">{{ t('columnTargetsOrder') }}</th>
            <th
              scope="col"
              class="tt-num"
            >
              {{ t('columnAfterLong') }}
            </th>
            <th
              scope="col"
              class="tt-num"
            >
              {{ t('columnMaxEventLag') }}
            </th>
            <th scope="col">{{ t('columnTrigger') }}</th>
            <th scope="col">
              <span class="tt-sr-only">{{ t('columnActions') }}</span>
            </th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="{ policy, trigger } in rows"
            :key="policy.source"
          >
            <td>
              <strong>{{ policy.source }}</strong>
            </td>
            <td>
              <StatusTag
                :variant="policy.enabled === false ? 'neutral' : 'success'"
                :label="
                  policy.enabled === false
                    ? t('statusDisabled')
                    : t('statusEnabled')
                "
              />
            </td>
            <td>
              <span class="failover-policies-tile__chips">
                <span
                  v-for="(target, index) in policy.targets ?? []"
                  :key="target"
                  class="failover-policies-tile__chip"
                  >{{ index + 1 }} · {{ target }}</span
                >
                <template v-if="!(policy.targets ?? []).length">{{
                  EMPTY_VALUE
                }}</template>
              </span>
            </td>
            <td class="tt-num">{{ afterText(policy) }}</td>
            <td class="tt-num">{{ lagText(policy) }}</td>
            <td>
              <span
                class="failover-policies-tile__trigger"
                :class="`failover-policies-tile__trigger--${trigger.state}`"
                >{{ triggerText(trigger) }}</span
              >
            </td>
            <td>
              <WriteGate>
                <span class="failover-policies-tile__actions">
                  <CButton
                    secondary
                    size="small"
                    @click="openEdit(policy)"
                  >
                    {{ t('edit') }}
                  </CButton>
                  <CButton
                    type="error"
                    ghost
                    size="small"
                    @click="deleting = policy"
                  >
                    {{ t('delete') }}
                  </CButton>
                </span>
              </WriteGate>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <FailoverPolicyDrawer
      v-model:show="showDrawer"
      :policy="editedPolicy"
    />

    <ConfirmDialog
      :show="deleting !== null"
      :title="t('deleteDialogTitle', { source: deleting?.source ?? '' })"
      :text="t('deleteConfirm', { source: deleting?.source ?? '' })"
      :confirm-label="t('deleteConfirmAction')"
      :cancel-label="t('cancel')"
      :loading="isDeleting"
      danger
      @update:show="deleting = null"
      @confirm="confirmDelete"
    />
  </SectionCard>
</template>

<style lang="scss" scoped>
  .failover-policies-tile {
    strong {
      font-weight: 600;
    }

    &__chips {
      display: inline-flex;
      flex-wrap: wrap;
      gap: 4px;
    }

    &__chip {
      padding: 1px 8px;
      font-size: 12px;
      border: 1px solid var(--tt-border-strong);
      border-radius: var(--tt-radius-tag);
    }

    &__trigger {
      font-size: 12px;
      font-weight: 600;

      &--yes {
        color: var(--tt-success);
      }

      &--no {
        color: var(--tt-danger);
      }

      &--disabled {
        color: var(--tt-text-3);
      }
    }

    &__actions {
      display: flex;
      justify-content: flex-end;
      gap: 8px;
    }
  }
</style>
