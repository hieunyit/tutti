<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  import { computed, h } from 'vue';
  import { RouterLink } from 'vue-router';
  import { useI18n } from 'vue-i18n';
  import { NButton, type DataTableColumns } from 'naive-ui';
  import {
    DataTable,
    EmptyBlock,
    FormSection,
    StatusBadge,
  } from '@/components/kit';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import messages from '@/components/tutti/users/i18nAccess';
  import { ProtectionGroupHelper } from '@/utils/helpers/ProtectionGroupHelper';
  import { RouteName } from '@/utils/types/router';
  import type { S3User } from '@/utils/types/users';
  import type { ProtectionGroup } from '@/utils/types/protection';

  defineProps<{
    user: S3User;
    groups: ProtectionGroup[];
    wildcard: boolean;
  }>();
  const emit = defineEmits<{ (e: 'edit'): void }>();

  const { t } = useI18n({ messages });

  const scopeKeys = {
    storage: 'scopeStorage',
    buckets: 'scopeBuckets',
  } as Record<string, string>;

  const columns = computed<DataTableColumns<ProtectionGroup>>(() => [
    {
      title: t('group'),
      key: 'id',
      render: (group) =>
        h(
          RouterLink,
          {
            to: {
              name: RouteName.TUTTI_PROTECTION_GROUP_DETAIL,
              params: { id: group.id },
            },
          },
          () => h('strong', group.id),
        ),
    },
    {
      title: t('scope'),
      key: 'scope',
      render: (group) =>
        t(scopeKeys[ProtectionGroupHelper.scopeKind(group)] ?? 'scopeUser'),
    },
    { title: t('primaryStorage'), key: 'primary' },
    {
      title: t('coveredBuckets'),
      key: 'buckets',
      render: (group) => h('code', group.buckets?.join(', ') || '–'),
    },
    {
      title: t('status'),
      key: 'state',
      render: (group) => {
        const state = ProtectionGroupHelper.displayState(group);

        return h(StatusBadge, {
          tone: ProtectionGroupHelper.stateVariant(state),
          label: state,
        });
      },
    },
  ]);
</script>

<template>
  <div class="user-perm">
    <FormSection
      :title="t('permission')"
      :description="t('policyHint')"
    >
      <div class="user-perm__row">
        <StatusBadge
          :tone="user.policy?.permission === 'read-only' ? 'info' : 'success'"
          :label="user.policy?.permission || 'read-write'"
        />
        <span
          >{{ t('summaryBuckets') }}:
          <code>{{ user.policy?.buckets?.join(', ') || '*' }}</code></span
        >
        <span
          v-if="wildcard"
          class="user-perm__muted"
          >{{ t('allBucketsNote') }}</span
        >
        <WriteGate>
          <NButton @click="emit('edit')">{{ t('editPolicy') }}</NButton>
        </WriteGate>
      </div>
    </FormSection>
    <FormSection
      :title="t('coverageTitle')"
      :description="t('coverageHint')"
    >
      <DataTable
        :columns="columns"
        :rows="groups"
        :loading="false"
        :row-key="(group: ProtectionGroup) => group.id"
      >
        <template #empty>
          <EmptyBlock :title="t('noGroupsTitle')" />
        </template>
      </DataTable>
    </FormSection>
  </div>
</template>

<style lang="scss" scoped>
  .user-perm {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-5);

    &__row {
      display: flex;
      flex-wrap: wrap;
      gap: var(--tt-space-3);
      align-items: center;
      font-size: var(--tt-text-sm);
    }

    &__muted {
      color: var(--tt-text-3);
    }
  }
</style>
