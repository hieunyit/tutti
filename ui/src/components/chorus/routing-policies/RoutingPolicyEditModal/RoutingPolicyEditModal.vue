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
  import { NAlert, NButton, NModal, NSelect, NSpace } from 'naive-ui';
  import { computed, ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nRoutingPolicies from '../i18nRoutingPolicies';
  import { ChorusService } from '@/services/ChorusService';
  import { useChorusRoutingPoliciesStore } from '@/stores/chorusRoutingPoliciesStore';
  import { useChorusNotification } from '@/utils/composables/useChorusNotification';
  import { ProxyAccountHelper } from '@/utils/helpers/ProxyAccountHelper';
  import type { RoutingPolicy } from '@/utils/types/chorus';

  const props = defineProps<{
    show: boolean;
    routingPolicy: RoutingPolicy;
  }>();

  const emit = defineEmits<{
    (e: 'update:show', value: boolean): void;
  }>();

  const { t } = useI18n({ messages: i18nRoutingPolicies });
  const { updateRoutingPolicyTarget } = useChorusRoutingPoliciesStore();
  const { createNotification } = useChorusNotification();

  const storageNames = ref<string[]>([]);
  const selected = ref<string | null>(null);
  const isLoading = ref(false);
  const isSaving = ref(false);
  const errorMessage = ref<string | null>(null);

  const options = computed(() =>
    storageNames.value
      .filter((name) => name !== props.routingPolicy.toStorage)
      .map((name) => ({ label: name, value: name })),
  );

  async function loadStorages() {
    isLoading.value = true;

    try {
      const { storages } = await ChorusService.getStorages();

      storageNames.value = storages.map((storage) => storage.name);
    } catch {
      errorMessage.value = t('editLoadError');
    } finally {
      isLoading.value = false;
    }
  }

  watch(
    () => props.show,
    (show) => {
      if (!show) return;

      selected.value = null;
      errorMessage.value = null;
      loadStorages();
    },
    // the parent mounts this modal with v-if when it opens, so show is already true
    { immediate: true },
  );

  async function submit() {
    if (!selected.value) return;

    isSaving.value = true;
    errorMessage.value = null;

    try {
      await updateRoutingPolicyTarget(props.routingPolicy, selected.value);

      createNotification({
        type: 'success',
        title: t('editSuccessTitle'),
        duration: 4000,
      });
      emit('update:show', false);
    } catch (error: unknown) {
      errorMessage.value =
        ProxyAccountHelper.getMessage(error) ?? t('editUnknownError');
    } finally {
      isSaving.value = false;
    }
  }
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    :title="t('editTitle')"
    :style="{ width: '480px', maxWidth: '95vw' }"
    :mask-closable="!isSaving"
    @update:show="emit('update:show', $event)"
  >
    <NSpace
      vertical
      :size="12"
    >
      <p>{{ t('editDescription') }}</p>
      <div>
        {{ routingPolicy.user }}
        <template v-if="routingPolicy.bucket !== '*'">
          / {{ routingPolicy.bucket }}
        </template>
        <br />
        <small>{{ t('editCurrent') }}: {{ routingPolicy.toStorage }}</small>
      </div>

      <NSelect
        v-model:value="selected"
        :options="options"
        :loading="isLoading"
        :placeholder="t('editPlaceholder')"
        :disabled="isSaving"
        :aria-label="t('editNew')"
      />
      <small v-if="!isLoading && options.length === 0">
        {{ t('editNoStorages') }}
      </small>

      <NAlert
        v-if="errorMessage"
        type="error"
      >
        {{ errorMessage }}
      </NAlert>
    </NSpace>

    <template #footer>
      <NSpace justify="end">
        <NButton
          :disabled="isSaving"
          @click="emit('update:show', false)"
        >
          {{ t('editCancel') }}
        </NButton>
        <NButton
          type="primary"
          :loading="isSaving"
          :disabled="!selected"
          @click="submit"
        >
          {{ t('editSave') }}
        </NButton>
      </NSpace>
    </template>
  </NModal>
</template>
