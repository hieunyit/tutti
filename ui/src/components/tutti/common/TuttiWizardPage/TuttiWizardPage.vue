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
   * Frame of an add page (replication, diff report, routing rule): title, a
   * back link to the list it came from, and the loading / error state.
   */
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import type { RouteLocationRaw } from 'vue-router';
  import i18nWizardPage from './i18nWizardPage';
  import { BackLink, PageShell } from '@/components/kit';
  import { RouteName } from '@/utils/types/router';

  const props = defineProps<{
    title: string;
    state: 'loading' | 'error' | 'ready';
    errorText?: string;
    /** list the user came from */
    backTo: 'replication' | 'diff' | 'routing';
  }>();

  defineEmits<{ (e: 'retry'): void }>();

  const { t } = useI18n({ messages: i18nWizardPage });

  const back = computed<{ to: RouteLocationRaw; label: string }>(() => {
    if (props.backTo === 'routing')
      return {
        to: { name: RouteName.TUTTI_ROUTING_RULES },
        label: t('backToRouting'),
      };

    return {
      to: {
        name: RouteName.TUTTI_REPLICATION,
        query: props.backTo === 'diff' ? { tab: 'diff' } : {},
      },
      label: t(props.backTo === 'diff' ? 'backToDiff' : 'backToReplication'),
    };
  });
</script>

<template>
  <PageShell
    :title="title"
    :state="state"
    :error-text="errorText"
    @retry="$emit('retry')"
  >
    <template #back>
      <BackLink
        :to="back.to"
        :label="back.label"
      />
    </template>

    <slot />
  </PageShell>
</template>
