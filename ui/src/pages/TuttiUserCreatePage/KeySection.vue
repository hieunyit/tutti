<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  import { useI18n } from 'vue-i18n';
  import { NCheckbox, NFormItem, NInput } from 'naive-ui';
  import { FormSection } from '@/components/kit';
  import i18nUsers from '@/components/tutti/users/i18nUsers';
  import { injectUserCreate } from '@/utils/composables/useUserCreate';

  const { t } = useI18n({ messages: i18nUsers });
  const { form } = injectUserCreate();
</script>

<template>
  <FormSection
    :title="t('keys')"
    :description="t('createKeyHint')"
  >
    <NCheckbox v-model:checked="form.createKey">{{
      t('createKeyNow')
    }}</NCheckbox>
    <NFormItem
      v-if="form.createKey"
      :label="t('keyDescription')"
    >
      <NInput
        v-model:value="form.description"
        :placeholder="t('keyDescriptionHint')"
      />
    </NFormItem>
  </FormSection>
</template>
