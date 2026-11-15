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
   * The one place for the actions of a row (pause or resume, delete): a
   * single "More" menu, write-gated. Actions come from useRowActions.
   */
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NButton, NDropdown } from 'naive-ui';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import i18nReplications from '@/components/tutti/replications/i18nReplications';
  import type { RowAction } from '@/utils/composables/useRowActions';

  const props = defineProps<{
    actions: RowAction[];
    /** names the row for screen readers ("More actions: prod to dr") */
    label: string;
  }>();

  const { t } = useI18n({ messages: i18nReplications });

  const options = computed(() =>
    props.actions.map((action) => ({
      key: action.key,
      label: action.label,
      disabled: action.busy || action.disabled,
    })),
  );

  function select(key: string) {
    props.actions.find((action) => action.key === key)?.invoke();
  }
</script>

<template>
  <WriteGate>
    <NDropdown
      trigger="click"
      placement="bottom-end"
      :options="options"
      @select="select"
    >
      <NButton
        quaternary
        size="small"
        :aria-label="`${t('moreActions')}: ${label}`"
      >
        <span aria-hidden="true">⋯</span>
      </NButton>
    </NDropdown>
  </WriteGate>
</template>
