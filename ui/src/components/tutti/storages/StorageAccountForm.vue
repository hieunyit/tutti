<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  import {
    NButton,
    NCheckbox,
    NCollapse,
    NCollapseItem,
    NFormItem,
    NInput,
  } from 'naive-ui';
  import { computed, onBeforeUnmount, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nStorages from './i18nStorages';
  import { StoragePolicyHelper } from '@/utils/helpers/StoragePolicyHelper';
  import type { TestOptionsForm } from '@/utils/helpers/StorageTestHelper';
  import type { StorageConnectionAccount } from '@/utils/types/runtimeStorage';
  import { RouteName } from '@/utils/types/router';

  const props = defineProps<{
    account: StorageConnectionAccount;
    options: TestOptionsForm;
    provider: string;
    readonly?: boolean;
    /** hide the feature and advanced controls (used by the rotate dialog) */
    compact?: boolean;
  }>();
  const emit = defineEmits<{
    (e: 'update:account', value: StorageConnectionAccount): void;
    (e: 'update:options', value: TestOptionsForm): void;
  }>();
  const { t } = useI18n({ messages: i18nStorages });
  const copied = ref<'idle' | 'ok' | 'failed'>('idle');
  let timer: ReturnType<typeof setTimeout> | undefined;
  const features = [
    'versioning',
    'tagging',
    'acl',
    'lifecycle',
    'policy',
  ] as const;
  const json = computed(() =>
    StoragePolicyHelper.generateJson(
      props.provider,
      props.options.features,
      props.options.policyPrefix.trim(),
    ),
  );
  const aclLeftOut = computed(
    () =>
      props.options.features.acl &&
      !StoragePolicyHelper.supportsAcl(props.provider),
  );

  function setAccount(key: keyof StorageConnectionAccount, value: string) {
    emit('update:account', { ...props.account, [key]: value });
  }

  function setOption<K extends keyof TestOptionsForm>(
    key: K,
    value: TestOptionsForm[K],
  ) {
    emit('update:options', { ...props.options, [key]: value });
  }

  function setFeature(key: (typeof features)[number], value: boolean) {
    setOption('features', { ...props.options.features, [key]: value });
  }

  async function copy() {
    try {
      await navigator.clipboard.writeText(json.value);
      copied.value = 'ok';
    } catch {
      copied.value = 'failed';
    }

    clearTimeout(timer);
    timer = setTimeout(() => {
      copied.value = 'idle';
    }, 2500);
  }

  onBeforeUnmount(() => clearTimeout(timer));
</script>

<template>
  <div class="account-form">
    <p class="account-form__hint">{{ t('accountHint') }}</p>
    <div class="account-form__pair">
      <NFormItem :label="t('accessKey')"
        ><NInput
          :value="account.accessKeyId"
          :disabled="readonly"
          autocomplete="off"
          @update:value="setAccount('accessKeyId', $event)"
      /></NFormItem>
      <NFormItem :label="t('secret')"
        ><NInput
          :value="account.secretAccessKey"
          type="password"
          show-password-on="click"
          :disabled="readonly"
          autocomplete="new-password"
          @update:value="setAccount('secretAccessKey', $event)"
      /></NFormItem>
    </div>
    <NFormItem
      v-if="!compact"
      :label="t('features')"
      ><div class="account-form__features">
        <NCheckbox
          v-for="feature in features"
          :key="feature"
          :checked="options.features[feature]"
          :disabled="readonly"
          @update:checked="setFeature(feature, $event)"
          >{{
            t(`feature${feature[0]!.toUpperCase()}${feature.slice(1)}`)
          }}</NCheckbox
        >
      </div></NFormItem
    >
    <div class="account-form__policy">
      <NButton
        secondary
        @click="copy"
        >{{ t('copyPolicy') }}</NButton
      >
      <RouterLink
        class="account-form__link"
        :to="{
          name: RouteName.TUTTI_DOCS,
          params: { slug: 'storage-accounts' },
        }"
        target="_blank"
        >{{ t('guideLink') }}</RouterLink
      >
      <span
        v-if="copied !== 'idle'"
        role="status"
        :class="copied === 'ok' ? 'account-form__ok' : 'account-form__failed'"
        >{{ copied === 'ok' ? t('policyCopied') : t('policyCopyFailed') }}</span
      >
    </div>
    <p
      v-if="aclLeftOut"
      class="account-form__hint"
    >
      {{ t('policyAclNote') }}
    </p>
    <details
      v-if="!compact"
      class="account-form__details"
    >
      <summary>{{ t('policyPreview') }}</summary>
      <pre class="account-form__json">{{ json }}</pre>
    </details>
    <NCollapse v-if="!compact">
      <NCollapseItem
        :title="t('advanced')"
        name="advanced"
      >
        <NFormItem
          :label="t('probePrefix')"
          :feedback="t('probePrefixHint')"
          ><NInput
            :value="options.probeBucketPrefix"
            :disabled="readonly"
            placeholder="tutti-probe-"
            @update:value="setOption('probeBucketPrefix', $event)"
        /></NFormItem>
        <NFormItem
          :label="t('probeBucket')"
          :feedback="t('probeBucketHint')"
          ><NInput
            :value="options.probeBucket"
            :disabled="readonly"
            @update:value="setOption('probeBucket', $event)"
        /></NFormItem>
        <NFormItem
          :label="t('policyPrefix')"
          :feedback="t('policyPrefixHint')"
          ><NInput
            :value="options.policyPrefix"
            :disabled="readonly"
            placeholder="app-"
            @update:value="setOption('policyPrefix', $event)"
        /></NFormItem>
        <NCheckbox
          :checked="options.throughput"
          :disabled="readonly"
          @update:checked="setOption('throughput', $event)"
          >{{ t('throughput') }}</NCheckbox
        >
      </NCollapseItem>
    </NCollapse>
  </div>
</template>

<style scoped lang="scss">
  .account-form {
    display: flex;
    flex-direction: column;
    gap: 8px;
    min-width: 0;

    &__hint {
      margin: 0;
      color: var(--tt-text-2);
      font-size: 13px;
    }

    &__pair {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 0 12px;
    }

    &__features,
    &__policy {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      gap: 8px 16px;
    }

    &__link {
      color: var(--tt-accent);
    }

    &__ok {
      color: var(--tt-success);
    }

    &__failed {
      color: var(--tt-danger);
      overflow-wrap: anywhere;
    }

    &__json {
      max-height: 240px;
      margin: 8px 0 0;
      padding: 12px;
      overflow: auto;
      font-size: 12px;
      background: var(--tt-subtle);
      border: 1px solid var(--tt-border);
      border-radius: 8px;
    }
    @media (max-width: 600px) {
      &__pair {
        grid-template-columns: minmax(0, 1fr);
      }
    }
  }
</style>
