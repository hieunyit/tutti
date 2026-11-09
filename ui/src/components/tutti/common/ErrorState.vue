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
  import { CButton } from '@clyso/clyso-ui-kit';
  import EmptyState from '@/components/ui/EmptyState.vue';
  import { IconName } from '@/utils/types/icon';

  defineProps<{
    title: string;
    text: string;
    actionLabel: string;
  }>();

  const emit = defineEmits<{
    (e: 'retry'): void;
  }>();
</script>

<template>
  <div
    class="tt-error-state"
    role="alert"
  >
    <EmptyState
      :icon="IconName.BASE_CLOSE_CIRCLE"
      :title="title"
      :text="text"
    >
      <CButton @click="emit('retry')">{{ actionLabel }}</CButton>
    </EmptyState>
  </div>
</template>

<style lang="scss" scoped>
  .tt-error-state {
    background-color: var(--tt-surface);
    border: 1px solid var(--tt-border);
    border-radius: var(--tt-radius-card);
  }
</style>
