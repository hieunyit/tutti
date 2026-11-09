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
  import { computed } from 'vue';
  import { calloutRole } from './status';

  defineOptions({ name: 'TtCallout' });

  const props = withDefaults(
    defineProps<{
      tone?: 'danger' | 'warning' | 'info';
      title?: string;
    }>(),
    { tone: 'info', title: undefined },
  );

  const role = computed(() => calloutRole(props.tone));
</script>

<template>
  <div
    class="tt-callout"
    :class="`tt-callout--${tone}`"
    :role="role"
  >
    <div class="tt-callout__content">
      <strong
        v-if="title"
        class="tt-callout__title"
        >{{ title }}</strong
      >
      <div class="tt-callout__body">
        <slot />
      </div>
    </div>
    <div
      v-if="$slots.actions"
      class="tt-callout__actions"
    >
      <slot name="actions" />
    </div>
  </div>
</template>

<style lang="scss" scoped>
  .tt-callout {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px 16px;
    padding: 10px 12px;
    border: 1px solid;
    border-radius: var(--tt-radius-control);
    font-size: 13px;
    line-height: 20px;
    color: var(--tt-text);

    &__content {
      display: flex;
      flex-direction: column;
      flex: 1 1 280px;
      min-width: 0;
    }

    &__title {
      font-weight: 600;
    }

    &__actions {
      display: flex;
      align-items: center;
      gap: 8px;
      margin-left: auto;
    }

    @each $name in (danger, warning, info) {
      &--#{$name} {
        background-color: var(--tt-#{$name}-bg);
        border-color: var(--tt-#{$name}-border);

        .tt-callout__title {
          color: var(--tt-#{$name});
        }
      }
    }

    :deep(a) {
      font-weight: 600;
      color: var(--tt-accent);
      text-decoration: underline;
      text-underline-offset: 2px;
    }
  }
</style>
