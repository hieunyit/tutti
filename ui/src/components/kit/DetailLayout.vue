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
   * Detail page: header (back link, name, status badge, actions), tabs, main
   * content and an optional side column (slot `aside`, stacks under 960px).
   * Built on PageShell, so it has the same four states.
   */
  import PageShell from './PageShell.vue';
  import StatusBadge from './StatusBadge.vue';
  import type { PageShellTab, PageState, StatusTone } from './types';

  withDefaults(
    defineProps<{
      title: string;
      description?: string;
      state: PageState;
      status?: { tone: StatusTone; label: string };
      tabs?: PageShellTab[];
      activeTab?: string;
      errorText?: string;
    }>(),
    {
      description: undefined,
      status: undefined,
      tabs: undefined,
      activeTab: undefined,
      errorText: undefined,
    },
  );

  const emit = defineEmits<{
    (e: 'update:activeTab', key: string): void;
    (e: 'retry'): void;
  }>();
</script>

<template>
  <PageShell
    :title="title"
    :description="description"
    :state="state"
    :tabs="tabs"
    :active-tab="activeTab"
    :error-text="errorText"
    @update:active-tab="emit('update:activeTab', $event)"
    @retry="emit('retry')"
  >
    <template
      v-if="$slots.back"
      #back
    >
      <slot name="back" />
    </template>
    <template
      v-if="status"
      #meta
    >
      <StatusBadge
        :tone="status.tone"
        :label="status.label"
      />
    </template>
    <template
      v-if="$slots.actions"
      #actions
    >
      <slot name="actions" />
    </template>
    <template
      v-if="$slots.empty"
      #empty
    >
      <slot name="empty" />
    </template>

    <div
      class="kit-detail"
      :class="{ 'kit-detail--aside': $slots.aside }"
    >
      <div class="kit-detail__main">
        <slot />
      </div>
      <aside
        v-if="$slots.aside"
        class="kit-detail__aside"
      >
        <slot name="aside" />
      </aside>
    </div>
  </PageShell>
</template>

<style lang="scss" scoped>
  .kit-detail {
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    gap: var(--tt-space-5);

    &--aside {
      grid-template-columns: minmax(0, 1fr) 320px;
    }

    &__main,
    &__aside {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-4);
      min-width: 0;
    }

    @media screen and (max-width: 959px) {
      &--aside {
        grid-template-columns: minmax(0, 1fr);
      }
    }
  }
</style>
