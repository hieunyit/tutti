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
   * One row action of a table: a small secondary icon button with a tooltip
   * and an accessible name. Destructive actions pass type="error" and are
   * wrapped in a confirmation by the caller.
   */
  import { CButton, CIcon, CTooltip } from '@clyso/clyso-ui-kit';
  import type { IconName } from '@/utils/types/icon';

  defineOptions({ inheritAttrs: false });
  const emit = defineEmits<{
    (e: 'click', event: MouseEvent): void;
  }>();

  withDefaults(
    defineProps<{
      label: string;
      icon: IconName;
      type?: 'default' | 'primary' | 'info' | 'success' | 'warning' | 'error';
      loading?: boolean;
      disabled?: boolean;
    }>(),
    {
      type: 'default',
    },
  );
</script>

<template>
  <span class="row-action-button">
    <CTooltip :delay="500">
      <template #trigger>
        <CButton
          v-bind="$attrs"
          secondary
          size="tiny"
          :type="type"
          :loading="loading"
          :disabled="disabled"
          :aria-label="label"
          @click="emit('click', $event)"
        >
          <template #icon>
            <CIcon
              :is-inline="true"
              :name="icon"
            />
          </template>
        </CButton>
      </template>

      {{ label }}
    </CTooltip>
  </span>
</template>

<style scoped>
  .row-action-button {
    display: inline-flex;
  }
</style>
