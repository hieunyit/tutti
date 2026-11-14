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
   * Replication detail (/replication/detail?from=..&to=..[&user=..&fromBucket=..&toBucket=..]):
   * name with inline rename, status and initial replication progress, the
   * copies running now and a console style activity log.
   */
  import { NButton } from 'naive-ui';
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { useRoute, useRouter } from 'vue-router';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import {
    BackLink,
    DetailLayout,
    EmptyBlock,
    SectionCard,
    type PageState,
  } from '@/components/kit';
  import ActivityConsole from '@/components/tutti/replications/activity/ActivityConsole.vue';
  import ActivityInFlightTable from '@/components/tutti/replications/activity/ActivityInFlightTable.vue';
  import i18nReplicationActivity from '@/components/tutti/replications/activity/i18nReplicationActivity';
  import ReplicationBucketCell from '@/components/tutti/replications/ReplicationBucketCell/ReplicationBucketCell.vue';
  import ReplicationDirectionCell from '@/components/tutti/replications/ReplicationDirectionCell/ReplicationDirectionCell.vue';
  import ReplicationNameCell from '@/components/tutti/replications/ReplicationNameCell/ReplicationNameCell.vue';
  import ReplicationStatusCell from '@/components/tutti/replications/ReplicationStatusCell/ReplicationStatusCell.vue';
  import { useNow } from '@/utils/composables/useNow';
  import { useReplicationActivity } from '@/utils/composables/useReplicationActivity';
  import { useReplicationDetail } from '@/utils/composables/useReplicationDetail';
  import { ReplicationDetailHelper } from '@/utils/helpers/ReplicationDetailHelper';
  import { ReplicationNameHelper } from '@/utils/helpers/ReplicationNameHelper';
  import { RouteName } from '@/utils/types/router';

  const { t } = useI18n({ messages: i18nReplicationActivity });
  const route = useRoute();
  const router = useRouter();

  const id = computed(() => ReplicationDetailHelper.fromQuery(route.query));
  const detail = useReplicationDetail(id);
  const activity = useReplicationActivity(id);
  const now = useNow(1000);

  void activity.start();

  const replication = detail.replication;
  const title = computed(() =>
    replication.value
      ? ReplicationNameHelper.getDisplayName(replication.value, '*')
      : (id.value?.fromStorage ?? ''),
  );
  const state = computed<PageState>(() => {
    if (!id.value || detail.missing.value) return 'empty';

    if (replication.value) return 'ready';

    return detail.error.value ? 'error' : 'loading';
  });
  const status = computed(() =>
    replication.value?.isPaused
      ? ({ tone: 'warning', label: t('statusPaused') } as const)
      : undefined,
  );

  function backToList() {
    router.push({ name: RouteName.TUTTI_REPLICATION });
  }
</script>

<template>
  <DetailLayout
    :title="title"
    :state="state"
    :status="status"
    :error-text="t('errorText')"
    @retry="detail.load()"
  >
    <template #back>
      <BackLink
        :to="{ name: RouteName.TUTTI_REPLICATION }"
        :label="t('back')"
      />
    </template>

    <template #actions>
      <template v-if="replication">
        <WriteGate>
          <NButton @click="detail.setPaused(!replication.isPaused)">{{
            replication.isPaused ? t('resume') : t('pause')
          }}</NButton>
        </WriteGate>
      </template>
    </template>

    <template #empty>
      <EmptyBlock
        :title="t('notFound')"
        :description="t('notFoundText')"
      >
        <NButton
          type="primary"
          @click="backToList"
          >{{ t('back') }}</NButton
        >
      </EmptyBlock>
    </template>

    <template v-if="replication">
      <SectionCard :title="t('overview')">
        <div class="replication-detail__overview">
          <ReplicationNameCell
            :replication="replication"
            :rename="detail.rename"
          />
          <ReplicationDirectionCell :replication="replication" />
          <ReplicationBucketCell :replication="replication" />
          <ReplicationStatusCell :replication="replication" />
        </div>
      </SectionCard>

      <SectionCard
        :title="t('inProgressTitle')"
        :meta="t('inProgressCount', { n: activity.inFlight.value.length })"
      >
        <ActivityInFlightTable
          :items="activity.inFlight.value"
          :now="now"
        />
      </SectionCard>

      <SectionCard :title="t('consoleTitle')">
        <ActivityConsole
          v-model:live="activity.live.value"
          :events="activity.events.value"
          :error="activity.error.value"
          :label="title"
          @clear="activity.clear()"
          @retry="activity.retry()"
        />
      </SectionCard>
    </template>
  </DetailLayout>
</template>

<style lang="scss" scoped>
  .replication-detail__overview {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    gap: var(--tt-space-4);
    align-items: start;
  }
</style>
