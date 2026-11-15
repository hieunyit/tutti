<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  /**
   * Side panel that sets the access policy of one user: permission, which
   * buckets (all, by protection group, or patterns) and a one-line preview.
   * Logic lives in useAccessPolicyForm; focus returns to the opener on close.
   */
  import {
    NAlert,
    NButton,
    NDrawer,
    NDrawerContent,
    NForm,
    NFormItem,
    NInput,
  } from 'naive-ui';
  import { computed, nextTick, toRef, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import PolicyCustomBuckets from './PolicyCustomBuckets.vue';
  import PolicyGrantMode from './PolicyGrantMode.vue';
  import PolicyGroupBuckets from './PolicyGroupBuckets.vue';
  import PolicyPermissionField from './PolicyPermissionField.vue';
  import PolicySummary from './PolicySummary.vue';
  import i18nPolicy from './i18nPolicy';
  import { useAccessPolicyForm } from './useAccessPolicyForm';
  import { useTuttiProtectionStore } from '@/stores/tuttiProtectionStore';
  import type { TuttiProxyAccount } from '@/utils/types/tutti';

  const props = defineProps<{
    show: boolean;
    account: TuttiProxyAccount | null;
  }>();
  const emit = defineEmits<{ (e: 'update:show', value: boolean): void }>();

  const { t } = useI18n({ messages: i18nPolicy });
  const protection = useTuttiProtectionStore();
  const policy = useAccessPolicyForm(
    toRef(props, 'show'),
    toRef(props, 'account'),
    () => emit('update:show', false),
  );
  const { form, patternErrors } = policy;

  const bucketError = computed(() =>
    patternErrors.value.length
      ? t(patternErrors.value[0] as string)
      : undefined,
  );

  // focus returns to whatever opened the drawer (Naive traps it while open)
  let opener: HTMLElement | null = null;

  watch(
    () => props.show,
    (open) => {
      if (open) {
        opener = document.activeElement as HTMLElement | null;
      } else {
        nextTick(() => opener?.focus?.());
      }
    },
    { flush: 'pre' },
  );
</script>

<template>
  <NDrawer
    :show="show"
    :trap-focus="true"
    width="min(540px, 100vw)"
    @update:show="emit('update:show', $event)"
  >
    <NDrawerContent
      :title="account?.hasPolicy ? t('editPolicy') : t('createPolicy')"
      closable
    >
      <NForm
        label-placement="top"
        @submit.prevent="policy.submit()"
      >
        <NFormItem :label="t('fieldUser')">
          <NInput
            :value="form.user"
            disabled
          />
        </NFormItem>

        <NFormItem :label="t('fieldPermission')">
          <PolicyPermissionField v-model="form.permission" />
        </NFormItem>

        <NFormItem :label="t('grantModeLabel')">
          <PolicyGrantMode
            :model-value="policy.grantMode.value"
            @update:model-value="policy.setGrantMode"
          />
        </NFormItem>

        <PolicyGroupBuckets
          v-if="policy.grantMode.value === 'groups'"
          v-model:extra-input="policy.extraInput.value"
          :groups="protection.groups"
          :selected-ids="policy.selectedGroupIds.value"
          :scope-groups="policy.scopeGroups.value"
          :buckets="policy.groupBuckets.value"
          :selected-buckets="policy.selectedGroupBuckets.value"
          :extra-buckets="policy.extraBuckets.value"
          @update:selected-ids="policy.setGroups"
          @toggle="policy.toggleGroupBucket"
          @all="policy.setAllGroupBuckets"
          @add-extra="policy.addExtra"
          @remove-extra="policy.removeExtra"
        />

        <NFormItem
          v-if="policy.grantMode.value === 'custom'"
          :label="t('fieldBuckets')"
          :validation-status="bucketError ? 'error' : undefined"
          :feedback="bucketError ?? t('fieldBucketsHint')"
        >
          <PolicyCustomBuckets
            v-model="policy.customInput.value"
            :patterns="form.buckets"
            :suggestions="policy.clusterBuckets.value"
            :add-disabled="
              policy.customInput.value === '' ||
              policy.patternError.value !== null
            "
            @add="policy.addCustom"
            @remove="policy.removeCustom"
            @suggest="policy.addSuggested"
          />
        </NFormItem>

        <NFormItem :label="t('fieldDescription')">
          <NInput
            v-model:value="form.description"
            type="textarea"
            :autosize="{ minRows: 2, maxRows: 4 }"
          />
        </NFormItem>
      </NForm>

      <PolicySummary :form="form" />

      <NAlert
        v-if="policy.saveError.value"
        type="error"
        class="policy-drawer__error"
      >
        {{
          policy.saveError.value === 'unknown'
            ? t('saveUnknownError')
            : policy.saveError.value
        }}
      </NAlert>

      <template #footer>
        <NButton
          class="policy-drawer__cancel"
          @click="emit('update:show', false)"
          >{{ t('cancel') }}</NButton
        >
        <NButton
          type="primary"
          :loading="policy.isSaving.value"
          @click="policy.submit()"
          >{{ t('save') }}</NButton
        >
      </template>
    </NDrawerContent>
  </NDrawer>
</template>

<style scoped lang="scss">
  .policy-drawer {
    &__error {
      margin-top: var(--tt-space-3);
    }

    &__cancel {
      margin-right: var(--tt-space-2);
    }
  }
</style>
