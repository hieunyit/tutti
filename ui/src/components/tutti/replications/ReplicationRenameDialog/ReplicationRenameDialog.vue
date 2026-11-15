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
  /** Rename dialog of the detail page: one optional name, saved on confirm. */
  import { ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { ConfirmDialog } from '@/components/kit';
  import ReplicationNameField from '@/components/tutti/replications/ReplicationNameField/ReplicationNameField.vue';
  import i18nReplicationName from '@/components/tutti/replications/i18nReplicationName';
  import { ReplicationNameHelper } from '@/utils/helpers/ReplicationNameHelper';

  const props = defineProps<{
    open: boolean;
    name?: string;
    /** persists the new (normalized) name; rejects when the server refuses it */
    save: (name: string) => Promise<void>;
  }>();

  const emit = defineEmits<{ (e: 'close'): void }>();

  const { t } = useI18n({ messages: i18nReplicationName });
  const draft = ref('');
  const saving = ref(false);
  const failed = ref(false);

  watch(
    () => props.open,
    (open) => {
      if (!open) return;

      draft.value = props.name ?? '';
      failed.value = false;
    },
  );

  async function confirm() {
    if (ReplicationNameHelper.getProblem(draft.value)) return;

    saving.value = true;
    failed.value = false;

    try {
      await props.save(ReplicationNameHelper.normalize(draft.value));
      emit('close');
    } catch {
      failed.value = true;
    } finally {
      saving.value = false;
    }
  }
</script>

<template>
  <ConfirmDialog
    :open="open"
    :title="t('renameLabel')"
    :confirm-label="t('renameSave')"
    :loading="saving"
    @confirm="confirm"
    @cancel="emit('close')"
  >
    <ReplicationNameField v-model="draft" />
    <p
      v-if="failed"
      role="alert"
    >
      {{ t('renameFailed') }}
    </p>
  </ConfirmDialog>
</template>
