<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  import {
    NCheckbox,
    NFormItem,
    NInput,
    NInputNumber,
    NSelect,
  } from 'naive-ui';
  import { useI18n } from 'vue-i18n';
  import i18nStorages from './i18nStorages';
  import type { RuntimeStorage } from '@/utils/types/runtimeStorage';

  const props = defineProps<{
    modelValue: RuntimeStorage;
    original?: RuntimeStorage;
    readonly?: boolean;
    showName?: boolean;
  }>();
  const emit = defineEmits<{
    (e: 'update:modelValue', value: RuntimeStorage): void;
  }>();
  const { t } = useI18n({ messages: i18nStorages });
  const providers = [
    { label: 'MinIO', value: 'Minio' },
    { label: 'Ceph', value: 'Ceph' },
    { label: 'Other S3', value: 'Other' },
  ];

  function set<K extends keyof RuntimeStorage>(
    key: K,
    value: RuntimeStorage[K],
  ) {
    emit('update:modelValue', { ...props.modelValue, [key]: value });
  }

  function rate(
    key: 'enabled' | 'rpm' | 'includeMetadataApi',
    value: boolean | number,
  ) {
    set('rateLimit', {
      enabled: false,
      rpm: 600,
      includeMetadataApi: false,
      ...props.modelValue.rateLimit,
      [key]: value,
    });
  }

  function changed(key: keyof RuntimeStorage) {
    return props.original && props.original[key] !== props.modelValue[key]
      ? t('changed', { value: String(props.original[key] || '–') })
      : undefined;
  }

  function tls(enabled: boolean) {
    const address = props.modelValue.address.replace(/^https?:\/\//, '');

    emit('update:modelValue', {
      ...props.modelValue,
      isSecure: enabled,
      address: `${enabled ? 'https' : 'http'}://${address}`,
    });
  }

  function rateChanged() {
    if (!props.original) return undefined;

    const normalize = (storage: RuntimeStorage) => ({
      enabled: storage.rateLimit?.enabled ?? false,
      rpm: storage.rateLimit?.rpm ?? 0,
      includeMetadataApi: storage.rateLimit?.includeMetadataApi ?? false,
    });
    const old = normalize(props.original);

    if (JSON.stringify(old) === JSON.stringify(normalize(props.modelValue)))
      return undefined;

    const value = old.enabled
      ? `${old.rpm} ${t('rpm')}${old.includeMetadataApi ? ` · ${t('metadata')}` : ''}`
      : t('rateOff');

    return t('changed', { value });
  }
</script>

<template>
  <div class="storage-form">
    <NFormItem
      :label="t('tls')"
      :feedback="t('tlsHint')"
      ><NCheckbox
        :checked="
          modelValue.address
            ? modelValue.address.startsWith('https://')
            : modelValue.isSecure
        "
        :disabled="readonly"
        @update:checked="tls"
        >HTTPS</NCheckbox
      ></NFormItem
    >
    <NFormItem
      v-if="showName"
      :label="t('name')"
      :feedback="t('nameHint')"
      ><NInput
        :value="modelValue.name"
        :disabled="readonly"
        autocomplete="off"
        @update:value="set('name', $event)"
    /></NFormItem>
    <NFormItem
      :label="t('provider')"
      :feedback="changed('provider')"
      ><NSelect
        :value="modelValue.provider"
        :options="providers"
        :disabled="readonly"
        @update:value="set('provider', $event)"
    /></NFormItem>
    <NFormItem
      class="storage-form__wide"
      :label="t('endpoint')"
      :feedback="changed('address')"
      ><NInput
        :value="modelValue.address"
        :disabled="readonly"
        placeholder="https://s3.example.com:9000"
        @update:value="set('address', $event)"
    /></NFormItem>
    <NFormItem
      :label="t('region')"
      :feedback="changed('defaultRegion')"
      ><NInput
        :value="modelValue.defaultRegion"
        :disabled="readonly"
        @update:value="set('defaultRegion', $event)"
    /></NFormItem>
    <NFormItem
      :label="t('timeout')"
      :feedback="changed('httpTimeout') || t('durationHint')"
      ><NInput
        :value="modelValue.httpTimeout"
        :disabled="readonly"
        @update:value="set('httpTimeout', $event)"
    /></NFormItem>
    <NFormItem
      :label="t('interval')"
      :feedback="changed('healthCheckInterval') || t('durationHint')"
      ><NInput
        :value="modelValue.healthCheckInterval"
        :disabled="readonly"
        @update:value="set('healthCheckInterval', $event)"
    /></NFormItem>
    <NFormItem
      :label="t('rate')"
      :feedback="rateChanged()"
      ><div class="storage-form__rate">
        <NCheckbox
          :checked="modelValue.rateLimit?.enabled ?? false"
          :disabled="readonly"
          @update:checked="rate('enabled', $event)"
          >{{ t('rate') }}</NCheckbox
        ><NInputNumber
          v-if="modelValue.rateLimit?.enabled"
          :value="modelValue.rateLimit.rpm"
          :min="1"
          :precision="0"
          :disabled="readonly"
          :aria-label="t('rpm')"
          @update:value="rate('rpm', $event ?? 0)"
        /><NCheckbox
          v-if="modelValue.rateLimit?.enabled"
          :checked="modelValue.rateLimit.includeMetadataApi"
          :disabled="readonly"
          @update:checked="rate('includeMetadataApi', $event)"
          >{{ t('metadata') }}</NCheckbox
        >
      </div></NFormItem
    >
  </div>
</template>

<style scoped lang="scss">
  .storage-form {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0 16px;

    &__wide {
      grid-column: 1 / -1;
    }

    &__rate {
      display: flex;
      flex-direction: column;
      gap: 10px;
    }
    @media (max-width: 600px) {
      grid-template-columns: minmax(0, 1fr);
    }
  }
</style>
