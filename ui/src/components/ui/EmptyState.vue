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
  import { CIcon } from '@clyso/clyso-ui-kit';
  import type { IconName } from '@/utils/types/icon';

  withDefaults(
    defineProps<{
      /** kit icon name */
      icon?: IconName;
      title?: string;
      text: string;
    }>(),
    { icon: undefined, title: undefined },
  );
</script>

<template>
  <div class="tt-empty">
    <span
      v-if="icon"
      class="tt-empty__icon"
      aria-hidden="true"
    >
      <CIcon
        :name="icon"
        :width="22"
        :height="22"
      />
    </span>
    <strong
      v-if="title"
      class="tt-empty__title"
      >{{ title }}</strong
    >
    <p class="tt-empty__text">{{ text }}</p>
    <div
      v-if="$slots.default"
      class="tt-empty__action"
    >
      <slot />
    </div>
  </div>
</template>

<style lang="scss" scoped>
  .tt-empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    padding: 32px 16px;
    text-align: center;

    &__icon {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: 44px;
      height: 44px;
      border-radius: 50%;
      color: var(--tt-text-3);
      background-color: var(--tt-subtle);
      border: 1px solid var(--tt-border);
    }

    &__title {
      font-size: 15px;
      font-weight: 600;
      color: var(--tt-text);
    }

    &__text {
      margin: 0;
      max-width: 44ch;
      font-size: 14px;
      color: var(--tt-text-2);
    }

    &__action {
      margin-top: 4px;
    }
  }
</style>
