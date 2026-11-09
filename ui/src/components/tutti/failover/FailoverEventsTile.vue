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
  import { CButton } from '@clyso/clyso-ui-kit';
  import { storeToRefs } from 'pinia';
  import { ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nFailover from './i18nFailover';
  import ConfirmDialog from '@/components/tutti/common/ConfirmDialog.vue';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import SectionCard from '@/components/ui/SectionCard.vue';
  import StatusTag from '@/components/ui/StatusTag.vue';
  import EmptyState from '@/components/ui/EmptyState.vue';
  import { IconName } from '@/utils/types/icon';
  import { useTuttiFailoverStore } from '@/stores/tuttiFailoverStore';
  import { useChorusNotification } from '@/utils/composables/useChorusNotification';
  import { formatDateTime } from '@/utils/helpers/DateTimeHelper';
  import type { TuttiFailoverEvent } from '@/utils/types/tutti';

  const { t } = useI18n({ messages: i18nFailover });
  const store = useTuttiFailoverStore();
  const { events } = storeToRefs(store);
  const { createNotification } = useChorusNotification();

  const acknowledging = ref<TuttiFailoverEvent | null>(null);
  const isSaving = ref(false);

  function scope(event: TuttiFailoverEvent): string {
    return event.bucket
      ? t('scopeBucket', { user: event.user, bucket: event.bucket })
      : t('scopeAllBuckets', { user: event.user });
  }

  async function confirmAcknowledge() {
    if (!acknowledging.value) return;

    isSaving.value = true;

    const ok = await store.acknowledgeEvent(acknowledging.value);

    isSaving.value = false;

    if (ok) {
      acknowledging.value = null;

      return;
    }

    createNotification({
      type: 'error',
      title: t('acknowledgeFailed'),
      duration: 6000,
    });
  }
</script>

<template>
  <SectionCard
    class="failover-events-tile"
    :title="t('eventsTitle')"
    :meta="t('eventsHintShort')"
    flush
  >
    <EmptyState
      v-if="events.length === 0"
      :icon="IconName.BASE_CHECKMARK"
      :title="t('eventsEmptyTitle')"
      :text="t('eventsEmptyText')"
    />

    <div
      v-else
      class="tt-table-wrap"
    >
      <table class="tt-table">
        <thead>
          <tr>
            <th scope="col">{{ t('columnWhen') }}</th>
            <th scope="col">{{ t('columnScope') }}</th>
            <th scope="col">{{ t('columnFromTo') }}</th>
            <th
              scope="col"
              class="tt-num"
            >
              {{ t('columnLag') }}
            </th>
            <th scope="col">{{ t('columnStatus') }}</th>
            <th scope="col">
              <span class="tt-sr-only">{{ t('columnActions') }}</span>
            </th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="event in events"
            :key="`${event.user}::${event.bucket ?? ''}`"
          >
            <td>{{ formatDateTime(event.time) }}</td>
            <td>{{ scope(event) }}</td>
            <td>{{ event.from }} → {{ event.to }}</td>
            <td class="tt-num">{{ event.eventLag ?? 0 }}</td>
            <td>
              <StatusTag
                variant="warning"
                :label="t('statusOpen')"
              />
            </td>
            <td class="failover-events-tile__actions">
              <WriteGate>
                <CButton
                  secondary
                  size="small"
                  @click="acknowledging = event"
                >
                  {{ t('acknowledge') }}
                </CButton>
              </WriteGate>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <ConfirmDialog
      :show="acknowledging !== null"
      :title="t('acknowledgeDialogTitle')"
      :text="
        acknowledging
          ? t('acknowledgeConfirm', {
              user: acknowledging.user,
              bucket: acknowledging.bucket || t('allBuckets'),
              from: acknowledging.from,
              to: acknowledging.to,
            })
          : ''
      "
      :confirm-label="t('acknowledge')"
      :cancel-label="t('cancel')"
      :loading="isSaving"
      @update:show="acknowledging = null"
      @confirm="confirmAcknowledge"
    />
  </SectionCard>
</template>

<style lang="scss" scoped>
  .failover-events-tile {
    &__actions {
      text-align: right;
    }
  }
</style>
