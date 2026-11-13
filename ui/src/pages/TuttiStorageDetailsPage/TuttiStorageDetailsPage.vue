<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  import { NButton } from 'naive-ui';
  import { computed, onBeforeMount, onBeforeUnmount } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { useRouter } from 'vue-router';
  import {
    BackLink,
    DetailLayout,
    EmptyBlock,
    useQueryTab,
    type StatusTone,
  } from '@/components/kit';
  import StorageHealthTab from '@/components/tutti/storages/StorageHealthTab.vue';
  import StorageOverviewTab from '@/components/tutti/storages/StorageOverviewTab.vue';
  import StorageSettings from '@/components/tutti/storages/StorageSettings.vue';
  import StorageUsersTab from '@/components/tutti/storages/StorageUsersTab.vue';
  import i18nStorages from '@/components/tutti/storages/i18nStorages';
  import { useTuttiStoragesStore } from '@/stores/tuttiStoragesStore';
  import { useNow } from '@/utils/composables/useNow';
  import { DashboardHelper } from '@/utils/helpers/DashboardHelper';
  import { StoragesPageHelper } from '@/utils/helpers/StoragesPageHelper';
  import { RouteName } from '@/utils/types/router';

  const props = defineProps<{ storageName: string }>();
  const { t } = useI18n({ messages: i18nStorages });
  const router = useRouter();
  const store = useTuttiStoragesStore();
  const now = useNow();
  const TABS = ['overview', 'health', 'users', 'settings'];
  const queryTab = useQueryTab('tab', 'overview');
  const tab = computed(() =>
    TABS.includes(queryTab.value) ? queryTab.value : 'overview',
  );

  const storage = computed(() =>
    store.runtimeStorages.find((item) => item.name === props.storageName),
  );
  const row = computed(() =>
    StoragesPageHelper.rows(
      {
        storages: store.storages,
        users: store.users,
        proxies: store.proxies,
        policies: store.policies,
        replications: store.replications,
        events: store.events,
      },
      now.value,
    ).find((item) => item.name === props.storageName),
  );
  const tabs = computed(() => [
    { key: 'overview', label: t('overview') },
    { key: 'health', label: t('healthByProxy') },
    {
      key: 'users',
      label: t('users'),
      badge: storage.value?.users?.length ?? 0,
    },
    { key: 'settings', label: t('settings') },
  ]);
  const health = computed(() =>
    store.proxies.map((proxy) => ({
      proxy,
      health: proxy.storages?.find((item) => item.name === props.storageName),
      fresh: DashboardHelper.isProxyReporting(proxy, now.value),
    })),
  );
  const status = computed(() => {
    const value = row.value?.health.status;

    if (!value) return undefined;

    const tone: StatusTone =
      value === 'available'
        ? 'success'
        : value === 'unavailable'
          ? 'danger'
          : 'neutral';

    return { tone, label: t(value) };
  });
  const state = computed(() => {
    if (!store.isLoaded) return store.hasError ? 'error' : 'loading';

    return storage.value ? 'ready' : 'empty';
  });
  const description = computed(() =>
    storage.value
      ? `${storage.value.address} · ${storage.value.provider} · ${
          storage.value.source === 'managed' ? t('managed') : t('config')
        }`
      : '',
  );

  onBeforeMount(store.startPolling);
  onBeforeUnmount(store.stopPolling);

  async function deleted() {
    await store.refresh();
    await router.push({ name: RouteName.CHORUS_STORAGES });
  }
</script>

<template>
  <DetailLayout
    :title="storageName"
    :description="description"
    :state="state"
    :status="status"
    :tabs="tabs"
    :active-tab="tab"
    :error-text="t('loadError')"
    @update:active-tab="queryTab = $event"
    @retry="store.refresh()"
  >
    <template #back>
      <BackLink
        :to="{ name: RouteName.CHORUS_STORAGES }"
        :label="t('backToStorages')"
      />
    </template>
    <template #actions>
      <NButton
        :loading="store.isLoading"
        @click="store.refresh()"
        >{{ t('refresh') }}</NButton
      >
    </template>
    <template #empty>
      <EmptyBlock :title="t('notFound')">
        <RouterLink :to="{ name: RouteName.CHORUS_STORAGES }">
          <NButton>{{ t('backToStorages') }}</NButton>
        </RouterLink>
      </EmptyBlock>
    </template>

    <template v-if="storage">
      <StorageOverviewTab
        v-if="tab === 'overview' && row"
        :row="row"
        :storage="storage"
      />
      <StorageHealthTab
        v-else-if="tab === 'health'"
        :rows="health"
        :now="now"
      />
      <StorageUsersTab
        v-else-if="tab === 'users'"
        :users="storage.users ?? []"
      />
      <StorageSettings
        v-else-if="tab === 'settings'"
        :key="storageName"
        :storage="storage"
        @saved="store.refresh()"
        @deleted="deleted"
      />
    </template>
  </DetailLayout>
</template>
