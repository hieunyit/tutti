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
   * Optional replication name input: visible label, the placeholder is only a
   * format hint, a live character counter, and the error shown after blur and
   * cleared as soon as the value is fixed.
   */
  import { computed, ref, useId } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NInput } from 'naive-ui';
  import i18nReplicationName from '@/components/tutti/replications/i18nReplicationName';
  import {
    REPLICATION_NAME_MAX_LENGTH,
    ReplicationNameHelper,
  } from '@/utils/helpers/ReplicationNameHelper';

  const { t } = useI18n({ messages: i18nReplicationName });
  const model = defineModel<string>({ required: true });
  const uid = useId();
  const touched = ref(false);

  const problem = computed(() => ReplicationNameHelper.getProblem(model.value));
  const error = computed(() => {
    if (!touched.value || !problem.value) return undefined;

    return problem.value === 'tooLong'
      ? t('nameTooLong')
      : t('nameControlChars');
  });
</script>

<template>
  <div class="replication-name-field">
    <label
      class="replication-name-field__label"
      :for="`replication-name-${uid}`"
      >{{ t('nameLabel') }}</label
    >
    <NInput
      v-model:value="model"
      :maxlength="REPLICATION_NAME_MAX_LENGTH"
      show-count
      :placeholder="t('namePlaceholder')"
      :status="error ? 'error' : undefined"
      :input-props="{
        id: `replication-name-${uid}`,
        autocomplete: 'off',
        'aria-describedby': `replication-name-help-${uid}`,
        'aria-invalid': error ? 'true' : undefined,
      }"
      @blur="touched = true"
    />
    <p
      :id="`replication-name-help-${uid}`"
      class="replication-name-field__help"
      :class="{ 'replication-name-field__help--error': error }"
      :role="error ? 'alert' : undefined"
    >
      {{ error ?? t('nameHint') }}
    </p>
  </div>
</template>

<style lang="scss" scoped>
  .replication-name-field {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-2);
    max-width: 420px;

    &__label {
      font-weight: 600;
    }

    &__help {
      margin: 0;
      font-size: 0.875rem;
      opacity: 0.8;

      &--error {
        opacity: 1;
        color: var(--tt-danger, #d03050);
      }
    }
  }
</style>
