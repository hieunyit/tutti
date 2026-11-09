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
  import { NModal } from 'naive-ui';

  withDefaults(
    defineProps<{
      show: boolean;
      title: string;
      text: string;
      confirmLabel: string;
      cancelLabel: string;
      /** red confirm button for destructive actions */
      danger?: boolean;
      loading?: boolean;
    }>(),
    { danger: false, loading: false },
  );

  const emit = defineEmits<{
    (e: 'update:show', value: boolean): void;
    (e: 'confirm'): void;
  }>();
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    :title="title"
    role="alertdialog"
    :style="{ width: '440px', maxWidth: '95vw' }"
    @update:show="emit('update:show', $event)"
  >
    <p class="tt-confirm__text">{{ text }}</p>

    <template #footer>
      <div class="tt-confirm__actions">
        <CButton
          :disabled="loading"
          @click="emit('update:show', false)"
        >
          {{ cancelLabel }}
        </CButton>
        <CButton
          :type="danger ? 'error' : 'primary'"
          :loading="loading"
          @click="emit('confirm')"
        >
          {{ confirmLabel }}
        </CButton>
      </div>
    </template>
  </NModal>
</template>

<style lang="scss" scoped>
  .tt-confirm {
    &__text {
      margin: 0;
      font-size: 14px;
      color: var(--tt-text-2);
    }

    &__actions {
      display: flex;
      justify-content: flex-end;
      gap: 8px;
    }
  }
</style>
