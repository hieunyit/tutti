<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  /** Grant by protection group: groups, their buckets (or a storage's buckets) and extra patterns. */
  import { NAlert, NButton, NCheckbox, NFormItem, NSelect } from 'naive-ui';
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import PolicyPatternList from './PolicyPatternList.vue';
  import i18nPolicy from './i18nPolicy';
  import type { GroupBucketItem } from './policyBuckets';
  import { ProtectionGroupHelper } from '@/utils/helpers/ProtectionGroupHelper';
  import type { ProtectionGroup } from '@/utils/types/protection';

  const props = defineProps<{
    groups: ProtectionGroup[];
    selectedIds: string[];
    scopeGroups: ProtectionGroup[];
    buckets: GroupBucketItem[];
    selectedBuckets: string[];
    extraBuckets: string[];
    extraInput: string;
  }>();
  const emit = defineEmits<{
    (e: 'update:selectedIds', value: string[]): void;
    (e: 'update:extraInput', value: string): void;
    (e: 'toggle', bucket: string, checked: boolean): void;
    (e: 'all', checked: boolean): void;
    (e: 'add-extra'): void;
    (e: 'remove-extra', pattern: string): void;
  }>();
  const { t } = useI18n({ messages: i18nPolicy });

  const storageOf = (group: ProtectionGroup) =>
    group.primary || (group.members ?? []).join(', ');
  const options = computed(() =>
    props.groups.map((group) => ({
      value: group.id,
      label: `${group.id} (${
        ProtectionGroupHelper.scopeKind(group) === 'storage'
          ? t('groupAllBucketsOn', { storage: storageOf(group) })
          : (group.buckets ?? []).join(', ') || t('groupNoBuckets')
      })`,
    })),
  );
</script>

<template>
  <NFormItem :label="t('groupsLabel')">
    <NSelect
      :value="selectedIds"
      multiple
      filterable
      :options="options"
      :placeholder="t('groupsPlaceholder')"
      @update:value="emit('update:selectedIds', $event)"
    />
  </NFormItem>

  <NAlert
    v-for="group in scopeGroups"
    :key="group.id"
    type="info"
    class="policy-groups__alert"
  >
    {{ t('storageScopeAlert', { group: group.id, storage: storageOf(group) }) }}
  </NAlert>

  <template v-if="selectedIds.length > 0">
    <NFormItem
      :label="t('groupBucketsLabel')"
      :feedback="t('groupBucketsHint')"
    >
      <div class="policy-groups__box">
        <template v-if="buckets.length > 0">
          <div class="policy-groups__bar">
            <NButton
              size="tiny"
              quaternary
              type="primary"
              @click="emit('all', true)"
              >{{ t('selectAllBuckets') }}</NButton
            >
            <NButton
              size="tiny"
              quaternary
              @click="emit('all', false)"
              >{{ t('deselectAllBuckets') }}</NButton
            >
          </div>
          <div class="policy-groups__grid">
            <NCheckbox
              v-for="item in buckets"
              :key="item.bucket"
              :checked="selectedBuckets.includes(item.bucket)"
              @update:checked="emit('toggle', item.bucket, $event)"
            >
              <span class="policy-groups__name">{{ item.bucket }}</span>
              <span class="policy-groups__origin"
                >({{
                  item.storage
                    ? t('originStorage', { storage: item.storage })
                    : item.groupId
                }})</span
              >
            </NCheckbox>
          </div>
        </template>
        <p
          v-else
          class="policy-groups__none"
        >
          {{ t('noBucketsInSelectedGroups') }}
        </p>
      </div>
    </NFormItem>

    <NFormItem
      :label="t('extraPatternsLabel')"
      :feedback="t('extraPatternsHint')"
    >
      <PolicyPatternList
        :patterns="extraBuckets"
        :model-value="extraInput"
        @update:model-value="emit('update:extraInput', $event)"
        @add="emit('add-extra')"
        @remove="emit('remove-extra', $event)"
      />
    </NFormItem>
  </template>
  <p
    v-else
    class="policy-groups__none"
  >
    {{ t('noGroupSelectedHint') }}
  </p>
</template>

<style scoped lang="scss">
  .policy-groups {
    &__alert {
      margin-bottom: var(--tt-space-3);
    }

    &__box {
      width: 100%;
      padding: var(--tt-space-3);
      border: 1px solid var(--tt-border);
      border-radius: var(--tt-radius-control);
      background: var(--tt-subtle);
    }

    &__bar {
      display: flex;
      gap: var(--tt-space-2);
      margin-bottom: var(--tt-space-2);
      padding-bottom: var(--tt-space-2);
      border-bottom: 1px solid var(--tt-border);
    }

    &__grid {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-2);
      max-height: 200px;
      overflow-y: auto;
    }

    &__name {
      margin-right: var(--tt-space-1);
      font-weight: var(--tt-weight-medium);
    }

    &__origin {
      font-size: var(--tt-text-xs);
      color: var(--tt-text-3);
    }

    &__none {
      margin: 0 0 var(--tt-space-3);
      font-size: var(--tt-text-sm);
      color: var(--tt-text-3);
    }
  }
</style>
