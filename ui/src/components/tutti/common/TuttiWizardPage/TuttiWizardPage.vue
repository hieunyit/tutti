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
  /** Frame of an Add* wizard page: title, back link, loading and error state. */
  import { useI18n } from 'vue-i18n';
  import { BackLink, PageShell } from '@/components/kit';
  import i18nReplicationHub from '@/pages/TuttiReplicationPage/i18nReplicationHub';
  import { RouteName } from '@/utils/types/router';

  defineProps<{
    title: string;
    description: string;
    state: 'loading' | 'error' | 'ready';
    errorText?: string;
    /** hub tab to go back to */
    backTab?: 'routing' | 'diff';
  }>();

  defineEmits<{ (e: 'retry'): void }>();

  const { t } = useI18n({ messages: i18nReplicationHub });
</script>

<template>
  <PageShell
    :title="title"
    :description="description"
    :state="state"
    :error-text="errorText"
    @retry="$emit('retry')"
  >
    <template #back>
      <BackLink
        :to="{
          name: RouteName.TUTTI_REPLICATION,
          query: backTab ? { tab: backTab } : {},
        }"
        :label="t('backToReplication')"
      />
    </template>

    <slot />
  </PageShell>
</template>
