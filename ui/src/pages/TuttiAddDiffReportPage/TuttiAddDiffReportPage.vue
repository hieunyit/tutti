<!--
  - Copyright © 2026 Clyso GmbH
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
  import { onBeforeMount, onUnmounted } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { CButton, CIcon } from '@clyso/clyso-ui-kit';
  import PageHeader from '@/components/layout/PageHeader.vue';
  import { RouteName } from '@/utils/types/router';
  import { IconName } from '@/utils/types/icon';
  import { useTuttiAddDiffReportStore } from '@/stores/tuttiAddDiffReportStore';
  import i18nAddDiffReport from '@/components/chorus/add-diff-report/i18nAddDiffReport';
  import AddDiffReportTile from '@/components/chorus/add-diff-report/AddDiffReportTile/AddDiffReportTile.vue';

  const { t } = useI18n({ messages: i18nAddDiffReport });
  const { initAddDiffReportPage, $reset } = useTuttiAddDiffReportStore();

  onBeforeMount(() => {
    initAddDiffReportPage();
  });

  onUnmounted(() => {
    $reset();
  });
</script>

<template>
  <div class="chorus-add-diff-report-page">
    <PageHeader
      :title="t('addDiffReportTitle')"
      :description="t('addDiffReportDescription')"
    >
      <template #actions>
        <RouterLink :to="{ name: RouteName.CHORUS_DIFF_REPORTS }">
          <CButton tag="div">
            <template #icon>
              <CIcon
                :is-inline="true"
                :name="IconName.BASE_ARROW_BACK"
              />
            </template>

            {{ t('backToDiffReports') }}
          </CButton>
        </RouterLink>
      </template>
    </PageHeader>

    <AddDiffReportTile />
  </div>
</template>
