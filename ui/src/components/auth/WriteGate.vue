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
   * Wraps a mutating control. Writers see the slot unchanged. Read-only accounts
   * see it greyed out and inert (no click, no keyboard focus) with a
   * "read-only account" tooltip, so a single wrapper covers buttons, links and
   * whole action groups.
   */
  import { NTooltip } from 'naive-ui';
  import { storeToRefs } from 'pinia';
  import { useI18n } from 'vue-i18n';
  import i18nAuth from './i18nAuth';
  import { useAuthStore } from '@/stores/authStore';

  defineProps<{
    /** Render the slot unchanged even for read-only accounts (non-mutating use). */
    bypass?: boolean;
  }>();

  const { t } = useI18n({ messages: i18nAuth });
  const { canWrite } = storeToRefs(useAuthStore());
</script>

<template>
  <slot v-if="canWrite || bypass" />
  <NTooltip v-else>
    <template #trigger>
      <span class="write-gate">
        <span
          class="write-gate__inner"
          inert
        >
          <slot />
        </span>
      </span>
    </template>
    {{ t('readOnlyAccount') }}
  </NTooltip>
</template>

<style lang="scss" scoped>
  .write-gate {
    display: inline-flex;
    cursor: not-allowed;

    &__inner {
      display: inline-flex;
      opacity: 0.45;
      pointer-events: none;
    }
  }
</style>
