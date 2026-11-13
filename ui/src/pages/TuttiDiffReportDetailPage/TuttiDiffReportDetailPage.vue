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
   * Diff report detail (/replication/diff/:id). The id is "<from>~<to>" (or any
   * old ?id=); the locations come from ?from & ?to, else from the id, so an old
   * deep link keeps working. Overview and entries are query-synced tabs.
   */
  import { computed, onBeforeMount, onUnmounted, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { storeToRefs } from 'pinia';
  import { useRoute, useRouter } from 'vue-router';
  import { NButton } from 'naive-ui';
  import {
    BackLink,
    ConfirmDialog,
    DetailLayout,
    EmptyBlock,
    useQueryTab,
    type PageShellTab,
    type StatusTone,
  } from '@/components/kit';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import DiffReportDetailEntries from '@/components/chorus/diff-report-detail/DiffReportDetailEntries/DiffReportDetailEntries.vue';
  import DiffReportDetailOverview from '@/components/chorus/diff-report-detail/DiffReportDetailOverview/DiffReportDetailOverview.vue';
  import DiffReportDetailProgress from '@/components/chorus/diff-report-detail/DiffReportDetailProgress/DiffReportDetailProgress.vue';
  import { parseDiffId } from '@/pages/TuttiReplicationPage/diffRoutes';
  import i18nReplicationHub from '@/pages/TuttiReplicationPage/i18nReplicationHub';
  import { useTuttiDiffReportDetailStore } from '@/stores/tuttiDiffReportDetailStore';
  import { useTuttiDiffReportEntriesStore } from '@/stores/tuttiDiffReportEntriesStore';
  import { useTuttiNotification } from '@/utils/composables/useTuttiNotification';
  import { DiffReportsHelper } from '@/utils/helpers/DiffReportsHelper';
  import { RouteName } from '@/utils/types/router';

  const { t } = useI18n({ messages: i18nReplicationHub });
  const route = useRoute();
  const router = useRouter();
  const store = useTuttiDiffReportDetailStore();
  const entriesStore = useTuttiDiffReportEntriesStore();
  const {
    isRestartLoading,
    hasError,
    isNotFound,
    locations,
    report,
    hasFixActivity,
  } = storeToRefs(store);
  const { createNotification } = useTuttiNotification();

  const activeTab = useQueryTab('tab', 'overview');
  const confirmRestart = ref(false);
  const missingLocations = ref(false);

  const tabs = computed<PageShellTab[]>(() => [
    { key: 'overview', label: t('tabOverview') },
    { key: 'entries', label: t('tabEntries') },
  ]);

  const state = computed(() => {
    if (missingLocations.value || isNotFound.value) return 'empty';

    if (hasError.value) return 'error';

    return report.value ? 'ready' : 'loading';
  });

  const status = computed<{ tone: StatusTone; label: string } | undefined>(
    () => {
      if (!report.value) return undefined;

      if (!report.value.ready)
        return { tone: 'info', label: t('statusChecking') };

      return report.value.consistent
        ? { tone: 'success', label: t('statusConsistent') }
        : { tone: 'danger', label: t('statusInconsistent') };
    },
  );

  const title = computed(() =>
    locations.value.length === 2
      ? locations.value
          .map((location) => `${location.storage}/${location.bucket}`)
          .join(' → ')
      : t('detailTitle'),
  );

  async function load() {
    const hasQuery =
      DiffReportsHelper.parseQueryParamsToLocations(
        route.query.from,
        route.query.to,
      ).length === 2;

    if (!hasQuery) {
      const fromId = parseDiffId(String(route.params.id ?? ''));

      if (!fromId) {
        missingLocations.value = true;

        return;
      }

      // the store reads the locations from the query: put them there
      await router.replace({
        params: route.params,
        query: {
          ...route.query,
          from: DiffReportsHelper.formatLocationForQuery(fromId[0]!),
          to: DiffReportsHelper.formatLocationForQuery(fromId[1]!),
        },
      });
    }

    missingLocations.value = false;
    await store.initDiffReportDetailsPage();
  }

  async function restart() {
    confirmRestart.value = false;

    try {
      await store.restartDiffReport();
      createNotification({
        type: 'success',
        title: t('actionDone'),
        duration: 4000,
      });
    } catch {
      createNotification({ type: 'error', title: t('actionFailed') });
    }
  }

  onBeforeMount(load);

  onUnmounted(() => {
    store.$reset();
    entriesStore.$reset();
  });
</script>

<template>
  <DetailLayout
    :title="title"
    :description="t('detailTitle')"
    :state="state"
    :status="status"
    :tabs="tabs"
    :active-tab="activeTab"
    :error-text="t('detailLoadError')"
    @update:active-tab="activeTab = $event"
    @retry="load"
  >
    <template #back>
      <BackLink
        :to="{ name: RouteName.CHORUS_REPLICATION, query: { tab: 'diff' } }"
        :label="t('backToDiffReports')"
      />
    </template>

    <template #actions>
      <WriteGate v-if="state === 'ready'">
        <NButton
          type="warning"
          :loading="isRestartLoading"
          @click="confirmRestart = true"
        >
          {{ t('restartReport') }}
        </NButton>
      </WriteGate>
    </template>

    <template #empty>
      <EmptyBlock
        :title="t('detailTitle')"
        :description="isNotFound ? t('detailNotFound') : t('detailMissingId')"
      />
    </template>

    <div
      v-if="activeTab === 'entries'"
      class="diff-detail__tab"
    >
      <DiffReportDetailEntries />
    </div>
    <div
      v-else
      class="diff-detail__tab"
    >
      <DiffReportDetailOverview />
      <DiffReportDetailProgress type="diff" />
      <DiffReportDetailProgress
        v-if="hasFixActivity"
        type="fix"
      />
    </div>

    <ConfirmDialog
      :open="confirmRestart"
      :title="t('confirmTitle', { action: t('restartReport') })"
      :confirm-label="t('restartReport')"
      @confirm="restart"
      @cancel="confirmRestart = false"
    >
      <p>{{ title }}</p>
    </ConfirmDialog>
  </DetailLayout>
</template>

<style lang="scss" scoped>
  .diff-detail__tab {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-4);
  }
</style>
