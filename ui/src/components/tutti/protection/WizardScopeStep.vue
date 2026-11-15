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
  import { NFormItem, NInput, NTag } from 'naive-ui';
  import { ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nProtection from './i18nProtection';
  import type {
    ProtectionFormError,
    ProtectionGroupForm,
  } from '@/utils/types/protection';

  defineProps<{
    errorText: (...codes: ProtectionFormError[]) => string | undefined;
  }>();
  const emit = defineEmits<{ (e: 'blur'): void }>();

  const form = defineModel<ProtectionGroupForm>({ required: true });
  const { t } = useI18n({ messages: i18nProtection });
  const bucketInput = ref('');

  /** Adds the typed bucket (Enter or blur); also called before saving. */
  function addBucket() {
    const value = bucketInput.value.trim();

    if (!value) return;

    form.value.buckets = [...form.value.buckets, value];
    bucketInput.value = '';
  }

  defineExpose({ addBucket });
</script>

<template>
  <div class="pg-scope">
    <NFormItem
      :label="t('fieldId')"
      :validation-status="
        errorText('ID_REQUIRED', 'ID_INVALID') ? 'error' : undefined
      "
      :feedback="errorText('ID_REQUIRED', 'ID_INVALID') ?? t('fieldIdHelp')"
    >
      <NInput
        v-model:value="form.id"
        @blur="emit('blur')"
      />
    </NFormItem>
    <NFormItem
      :label="t('fieldBuckets')"
      :validation-status="errorText('BUCKET_INVALID') ? 'error' : undefined"
      :feedback="errorText('BUCKET_INVALID') ?? t('fieldBucketsHelp')"
    >
      <div class="pg-scope__stack">
        <div
          v-if="form.buckets.length"
          class="pg-scope__row"
        >
          <NTag
            v-for="(bucket, index) in form.buckets"
            :key="`${bucket}-${index}`"
            round
            closable
            @close="form.buckets = form.buckets.filter((_, i) => i !== index)"
          >
            {{ bucket }}
          </NTag>
        </div>
        <NInput
          v-model:value="bucketInput"
          :placeholder="t('fieldBucketsPlaceholder')"
          @keydown.enter.prevent="addBucket"
          @blur="addBucket"
        />
      </div>
    </NFormItem>
  </div>
</template>

<style lang="scss" scoped>
  .pg-scope {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-3);

    &__stack {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-2);
      width: 100%;
    }

    &__row {
      display: flex;
      flex-wrap: wrap;
      gap: var(--tt-space-2);
    }
  }
</style>
