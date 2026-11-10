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
  import { useI18n } from 'vue-i18n';
  import { CIcon, CButton } from '@clyso/clyso-ui-kit';
  import { storeToRefs } from 'pinia';
  import { useRouter } from 'vue-router';
  import i18nDiffReports from '@/components/chorus/diff-reports/i18nDiffReports';
  import EmptyState from '@/components/ui/EmptyState.vue';
  import { IconName } from '@/utils/types/icon';
  import { useTuttiDiffReportsStore } from '@/stores/tuttiDiffReportsStore';
  import { RouteName } from '@/utils/types/router';
  import WriteGate from '@/components/auth/WriteGate.vue';

  const { t } = useI18n({
    messages: i18nDiffReports,
  });

  const { isFiltered } = storeToRefs(useTuttiDiffReportsStore());
  const { clearFilters } = useTuttiDiffReportsStore();
  const router = useRouter();

  function handleAction() {
    if (!isFiltered.value) {
      router.push({ name: RouteName.CHORUS_ADD_DIFF_REPORT });

      return;
    }

    clearFilters();
  }
</script>

<template>
  <EmptyState
    :icon="IconName.BASE_DOCUMENT"
    :title="t(isFiltered ? 'filterNoResultsTitle' : 'noResultsTitle')"
    :text="t(isFiltered ? 'filterNoResultsText' : 'noResultsText')"
  >
    <WriteGate :bypass="isFiltered">
      <CButton
        :type="isFiltered ? 'default' : 'primary'"
        @click="handleAction"
      >
        <template #icon>
          <CIcon
            :is-inline="true"
            :name="isFiltered ? 'close' : IconName.BASE_ADD"
          />
        </template>

        {{ t(isFiltered ? 'filterNoResultsAction' : 'noResultsAction') }}
      </CButton>
    </WriteGate>
  </EmptyState>
</template>
