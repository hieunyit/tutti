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
   * header (name, source to destination, status, rename and pause or resume),
   * progress summary, the copies running now and a console style activity log.
   */
  import { NButton } from 'naive-ui';
  import { computed, ref } from 'vue';
  import { useNotification } from '@clyso/clyso-ui-kit';
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
  import ReplicationRenameDialog from '@/components/tutti/replications/ReplicationRenameDialog/ReplicationRenameDialog.vue';
  import ReplicationSummary from '@/components/tutti/replications/ReplicationSummary.vue';
  import ReplicationPoliciesCard from '@/components/tutti/replications/ReplicationPoliciesCard.vue';
  import i18nReplicationRescan from '@/components/tutti/replications/i18nReplicationRescan';
  import i18nReplications from '@/components/tutti/replications/i18nReplications';
  import { getReplicationStatus } from '@/components/tutti/replications/replicationStatus';
  import { mergeMessages } from '@/utils/helpers/mergeMessages';
  import { useNow } from '@/utils/composables/useNow';
  import { useReplicationActivity } from '@/utils/composables/useReplicationActivity';
  import { useReplicationDetail } from '@/utils/composables/useReplicationDetail';
  import { ReplicationDetailHelper } from '@/utils/helpers/ReplicationDetailHelper';
  import { ReplicationNameHelper } from '@/utils/helpers/ReplicationNameHelper';
  import { RouteName } from '@/utils/types/router';

  const { t } = useI18n({
    messages: mergeMessages(
      i18nReplications,
      i18nReplicationActivity,
      i18nReplicationRescan,
    ),
  });
  const route = useRoute();
  const router = useRouter();

  const id = computed(() => ReplicationDetailHelper.fromQuery(route.query));
  const detail = useReplicationDetail(id);
  const replication = detail.replication;
  const activity = useReplicationActivity(id);
  const now = useNow(1000);
  const renaming = ref(false);
  const pausing = ref(false);
  const { createNotification } = useNotification();

  async function handleTogglePause() {
    if (!replication.value) return;

    const willPause = !replication.value.isPaused;

    pausing.value = true;

    try {
      await detail.setPaused(willPause);

      createNotification({
        type: 'success',
        title: willPause ? t('pauseSuccessTitle') : t('resumeSuccessTitle'),
      });
    } catch (err: unknown) {
      createNotification({
        type: 'error',
        title: willPause ? t('pauseErrorTitle') : t('resumeErrorTitle'),
        content: err instanceof Error ? err.message : t('errorText'),
      });
    } finally {
      pausing.value = false;
    }
  }

  void activity.start();

  const title = computed(() =>
    replication.value
      ? ReplicationNameHelper.getDisplayName(replication.value, t('allBuckets'))
      : (id.value?.fromStorage ?? ''),
  );

  // "source → destination · bucket" under a custom name; the auto name already says it
  const description = computed(() =>
    replication.value
      ? ReplicationNameHelper.getSecondaryLabel(
          replication.value,
          t('allBuckets'),
        ) || undefined
      : undefined,
  );

  const state = computed<PageState>(() => {
    if (!id.value || detail.missing.value) return 'empty';

    if (replication.value) return 'ready';

    return detail.error.value ? 'error' : 'loading';
  });

  const status = computed(() => {
    if (!replication.value) return undefined;

    const { tone, label } = getReplicationStatus(replication.value, t);

    return { tone, label };
  });

  function backToList() {
    router.push({ name: RouteName.TUTTI_REPLICATION });
  }
</script>

<template>
  <DetailLayout
    :title="title"
    :description="description"
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
      <WriteGate v-if="replication">
        <NButton @click="renaming = true">{{ t('rename') }}</NButton>
        <NButton
          :loading="pausing"
          :disabled="pausing"
          @click="handleTogglePause"
        >{{ replication.isPaused ? t('resume') : t('pause') }}</NButton>
      </WriteGate>
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
      <ReplicationSummary
        :replication="replication"
        :sync="detail.syncNow"
        :change-interval="detail.setRescanInterval"
      />
      <ReplicationPoliciesCard
        :replication="replication"
        :reload="detail.refresh"
      />
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
      <ReplicationRenameDialog
        :open="renaming"
        :name="replication.name"
        :save="detail.rename"
        @close="renaming = false"
      />
    </template>
  </DetailLayout>
</template>
