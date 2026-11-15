<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  import { computed, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NFormItem, NSelect } from 'naive-ui';
  import { FormSection } from '@/components/kit';
  import PolicyCustomBuckets from '@/components/tutti/accounts/PolicyCustomBuckets.vue';
  import PolicyGrantMode from '@/components/tutti/accounts/PolicyGrantMode.vue';
  import PolicyGroupBuckets from '@/components/tutti/accounts/PolicyGroupBuckets.vue';
  import i18nUsers from '@/components/tutti/users/i18nUsers';
  import { useTuttiProtectionStore } from '@/stores/tuttiProtectionStore';
  import {
    injectUserCreate,
    type GrantMode,
  } from '@/utils/composables/useUserCreate';
  import { AccessPolicyHelper } from '@/utils/helpers/AccessPolicyHelper';
  import { UserHelper } from '@/utils/helpers/UserHelper';

  const props = defineProps<{ showErrors: boolean }>();
  const { t } = useI18n({ messages: i18nUsers });
  const ctl = injectUserCreate();
  const { form } = ctl;
  const protection = useTuttiProtectionStore();
  const extraInput = ref('');
  const customInput = ref('');
  const extraError = ref('');

  const permissionOptions = computed(() => [
    { label: t('readWrite'), value: 'read-write' },
    { label: t('readOnly'), value: 'read-only' },
    { label: t('writeOnly'), value: 'write-only' },
  ]);
  const customPatterns = computed(
    () => UserHelper.parseBucketPatterns(form.value.buckets).patterns,
  );
  const problem = computed(() =>
    props.showErrors && ctl.permissionProblem.value
      ? t(ctl.permissionProblem.value)
      : extraError.value || undefined,
  );

  function selectMode(mode: GrantMode) {
    form.value.grantMode = mode;

    if (mode === 'all') form.value.buckets = '*';
    else if (mode === 'custom' && form.value.buckets === '*') {
      form.value.buckets = '';
    }
  }

  function addExtra() {
    const key = ctl.addExtraPattern(extraInput.value);

    extraError.value = key ? t(key) : '';

    if (!key) extraInput.value = '';
  }

  function addCustom() {
    const pattern = customInput.value.trim();

    if (!pattern) return;

    if (AccessPolicyHelper.validatePattern(pattern)) {
      extraError.value = t('invalidBuckets');

      return;
    }

    extraError.value = '';
    ctl.addSuggestedBucket(pattern);
    customInput.value = '';
  }

  function removeCustom(pattern: string) {
    form.value.buckets = customPatterns.value
      .filter((p) => p !== pattern)
      .join(', ');
  }
</script>

<template>
  <FormSection
    :title="t('createSectionAccess')"
    :error="problem"
  >
    <NFormItem :label="t('permission')">
      <NSelect
        v-model:value="form.permission"
        class="access-section__select"
        :options="permissionOptions"
      />
    </NFormItem>
    <NFormItem :label="t('grantModeLabel')">
      <PolicyGrantMode
        :model-value="form.grantMode"
        @update:model-value="selectMode"
      />
    </NFormItem>

    <PolicyGroupBuckets
      v-if="form.grantMode === 'groups'"
      v-model:extra-input="extraInput"
      :groups="protection.groups"
      :selected-ids="form.selectedGroupIds"
      :scope-groups="ctl.storageGroups.value"
      :buckets="ctl.availableGroupBuckets.value"
      :selected-buckets="form.selectedGroupBuckets"
      :extra-buckets="form.customExtraBuckets"
      @update:selected-ids="ctl.setGroups"
      @toggle="ctl.setGroupBucket"
      @all="ctl.selectAllGroupBuckets"
      @add-extra="addExtra"
      @remove-extra="ctl.removeExtraPattern"
    />

    <NFormItem
      v-if="form.grantMode === 'custom'"
      :label="t('allowedBuckets')"
      :feedback="t('bucketsHint')"
    >
      <PolicyCustomBuckets
        v-model="customInput"
        :patterns="customPatterns"
        :suggestions="ctl.clusterBuckets.value"
        :add-disabled="customInput.trim() === ''"
        @add="addCustom"
        @remove="removeCustom"
        @suggest="ctl.addSuggestedBucket"
      />
    </NFormItem>
  </FormSection>
</template>

<style lang="scss" scoped>
  .access-section__select {
    max-width: 280px;
  }
</style>
