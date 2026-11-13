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
  /** Row action buttons built by useRowActions().actionsFor(row); write-gated. */
  import { NButton } from 'naive-ui';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import type { RowAction } from '@/utils/composables/useRowActions';

  defineProps<{ actions: RowAction[] }>();
</script>

<template>
  <WriteGate>
    <div class="row-actions">
      <span
        v-for="action in actions"
        :key="action.key"
        :title="action.disabledReason"
      >
        <NButton
          size="tiny"
          secondary
          :type="action.danger ? 'error' : 'default'"
          :loading="action.busy"
          :disabled="action.busy || action.disabled"
          @click.stop="action.invoke()"
        >
          {{ action.label }}
        </NButton>
      </span>
    </div>
  </WriteGate>
</template>

<style lang="scss" scoped>
  .row-actions {
    display: flex;
    flex-wrap: wrap;
    gap: var(--tt-space-2);
  }
</style>
