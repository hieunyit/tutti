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

<script setup lang="ts" generic="T">
  /** ConfirmDialog bound to useRowActions().pending (names the target). */
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { ConfirmDialog } from '@/components/kit';
  import type { PendingRowAction } from '@/utils/composables/useRowActions';
  import i18nReplicationHub from '@/pages/TuttiReplicationPage/i18nReplicationHub';

  const props = defineProps<{
    pending: PendingRowAction<T> | null;
    target: (row: T) => string;
  }>();

  defineEmits<{ (e: 'confirm'): void; (e: 'cancel'): void }>();

  const { t } = useI18n({ messages: i18nReplicationHub });
  const label = computed(() => props.pending?.def.label ?? '');
</script>

<template>
  <ConfirmDialog
    :open="pending !== null"
    :title="t('confirmTitle', { action: label })"
    :target-name="pending ? target(pending.row) : undefined"
    danger
    :confirm-label="label"
    @confirm="$emit('confirm')"
    @cancel="$emit('cancel')"
  >
    <p>{{ pending?.message }}</p>
  </ConfirmDialog>
</template>
