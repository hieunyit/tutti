<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NButton } from 'naive-ui';
  import { RouterLink } from 'vue-router';
  import CredentialsTab from './CredentialsTab.vue';
  import UserDeleteDialog from './UserDeleteDialog.vue';
  import PermissionsTab from './PermissionsTab.vue';
  import {
    BackLink,
    Callout,
    DetailLayout,
    EmptyBlock,
    SectionCard,
  } from '@/components/kit';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import ProxySecretModal from '@/components/tutti/accounts/ProxySecretModal.vue';
  import AccessPolicyDrawer from '@/components/tutti/accounts/AccessPolicyDrawer.vue';
  import messages from '@/components/tutti/users/i18nUsers';
  import { keyCount, keysLink } from '@/components/tutti/users/userColumns';
  import { useTuttiUsersStore } from '@/stores/tuttiUsersStore';
  import { useTuttiAccessPolicyStore } from '@/stores/tuttiAccessPolicyStore';
  import { useTuttiProtectionStore } from '@/stores/tuttiProtectionStore';
  import { UserHelper } from '@/utils/helpers/UserHelper';
  import { ProxyAccountHelper } from '@/utils/helpers/ProxyAccountHelper';
  import type { TuttiProxyAccountSecret } from '@/utils/types/tutti';
  import { RouteName } from '@/utils/types/router';

  const props = defineProps<{ name: string }>();
  const { t } = useI18n({ messages });
  const store = useTuttiUsersStore();
  const keys = useTuttiAccessPolicyStore();
  const protection = useTuttiProtectionStore();

  const user = computed(() => store.find(props.name));
  const account = computed(
    () => keys.accounts.find((a) => a.user === props.name) ?? null,
  );
  const isConfig = computed(
    () => user.value?.storages?.some((s) => s.source === 'config') ?? false,
  );
  const buckets = computed(() => user.value?.policy?.buckets ?? []);
  const wildcard = computed(
    () => buckets.value.length === 0 || buckets.value.includes('*'),
  );

  const groups = computed(() =>
    protection.groups.filter(
      (g) =>
        g.user === props.name ||
        wildcard.value ||
        (g.buckets ?? []).some((b) => buckets.value.includes(b)),
    ),
  );

  /** storages this user can reach: overrides, plus connection accounts in scope */
  const storages = computed(() => {
    const scoped = new Set<string>();

    for (const group of groups.value) {
      if (group.primary) scoped.add(group.primary);

      (group.members ?? []).forEach((m) => m && scoped.add(m));
    }

    return (user.value?.storages ?? []).filter((entry) => {
      const mode = UserHelper.modeOf(entry);

      if (mode === 'override') return true;

      if (mode === 'missing') return false;

      return wildcard.value || scoped.has(entry.storage);
    });
  });

  const state = computed(() => {
    if (store.hasError || keys.hasError) return 'error' as const;

    if (!store.isLoaded) return 'loading' as const;

    return user.value ? ('ready' as const) : ('empty' as const);
  });
  const busy = ref(false);
  const error = ref('');
  const showPolicy = ref(false);
  /** one-time key secret: memory only, cleared on close */
  const secret = ref<TuttiProxyAccountSecret | null>(null);

  async function load() {
    await Promise.all([
      store.load(),
      keys.init(),
      protection.refresh().catch(() => null),
    ]);
  }

  async function createKey() {
    busy.value = true;
    error.value = '';

    try {
      secret.value = await keys.createAccount(props.name, '');
    } catch (e) {
      error.value = ProxyAccountHelper.getMessage(e) ?? t('error');
    } finally {
      busy.value = false;
    }
  }

  function closeSecret() {
    secret.value = null;
    load();
  }

  onMounted(load);
  watch(
    () => props.name,
    () => {
      error.value = '';
      load();
    },
  );
  onBeforeUnmount(() => {
    secret.value = null;
  });
</script>

<template>
  <DetailLayout
    :title="name"
    :state="state"
    :status="
      user && isConfig ? { tone: 'neutral', label: t('config') } : undefined
    "
    @retry="load"
  >
    <template #back>
      <BackLink
        :to="{ name: RouteName.TUTTI_USERS }"
        :label="t('users')"
      />
    </template>
    <template #actions>
      <RouterLink
        v-if="user"
        :to="keysLink(name)"
      >
        <NButton>{{
          t('keysCount', { count: keyCount(user) }, keyCount(user))
        }}</NButton>
      </RouterLink>
      <WriteGate>
        <NButton
          type="primary"
          :loading="busy"
          :disabled="!user || !store.dynamicEnabled"
          @click="createKey"
          >{{ t('newKey') }}</NButton
        >
      </WriteGate>
    </template>
    <template #empty>
      <EmptyBlock
        :title="t('userNotFound')"
        :description="t('userNotFoundHint')"
      >
        <RouterLink :to="{ name: RouteName.TUTTI_USERS }">
          <NButton>{{ t('backToUsers') }}</NButton>
        </RouterLink>
      </EmptyBlock>
    </template>

    <div
      v-if="user"
      class="user-detail"
    >
      <Callout
        v-if="error"
        tone="danger"
        >{{ error }}</Callout
      >
      <SectionCard
        :title="t('storages')"
        variant="section"
      >
        <CredentialsTab
          :user-name="name"
          :storages="storages"
          :dynamic-enabled="store.dynamicEnabled"
          @changed="load"
        />
      </SectionCard>
      <SectionCard
        :title="t('accessPolicy')"
        variant="section"
      >
        <PermissionsTab
          :user="user"
          :groups="groups"
          @edit="showPolicy = true"
        />
      </SectionCard>

      <WriteGate>
        <UserDeleteDialog
          :name="name"
          :disabled="isConfig"
        />
      </WriteGate>
    </div>

    <AccessPolicyDrawer
      v-model:show="showPolicy"
      :account="account ?? { user: name }"
      @update:show="!$event && load()"
    />
    <ProxySecretModal
      :secret="secret"
      kind="created"
      @close="closeSecret"
    />
  </DetailLayout>
</template>

<style lang="scss" scoped>
  .user-detail {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-6);
  }
</style>
