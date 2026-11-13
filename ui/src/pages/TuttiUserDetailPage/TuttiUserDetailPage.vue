<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { useRouter } from 'vue-router';
  import { NButton, NCheckbox } from 'naive-ui';
  import CredentialsTab from './CredentialsTab.vue';
  import PermissionsTab from './PermissionsTab.vue';
  import {
    BackLink,
    Callout,
    ConfirmDialog,
    DetailLayout,
    EmptyBlock,
    FormSection,
    useQueryTab,
  } from '@/components/kit';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import ProxyAccountKeysTable from '@/components/tutti/accounts/ProxyAccountKeysTable.vue';
  import ProxyKeyRotateModal from '@/components/tutti/accounts/ProxyKeyRotateModal.vue';
  import ProxySecretModal from '@/components/tutti/accounts/ProxySecretModal.vue';
  import AccessPolicyDrawer from '@/components/tutti/accounts/AccessPolicyDrawer.vue';
  import messages from '@/components/tutti/users/i18nAccess';
  import { TuttiService } from '@/services/TuttiService';
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
  const router = useRouter();
  const tab = useQueryTab('tab', 'credentials');

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
  const tabs = computed(() => [
    { key: 'credentials', label: t('credentials') },
    {
      key: 'keys',
      label: t('keys'),
      badge: account.value
        ? ProxyAccountHelper.getKeyRows(account.value).length
        : 0,
    },
    { key: 'permissions', label: t('permission') },
  ]);

  const busy = ref(false);
  const error = ref('');
  const showDelete = ref(false);
  const force = ref(false);
  const canForce = ref(false);
  const showPolicy = ref(false);
  /** one-time key secret: memory only, cleared on close */
  const secret = ref<TuttiProxyAccountSecret | null>(null);
  const secretKind = ref<'created' | 'rotated'>('created');
  const rotateId = ref<string | null>(null);

  async function load() {
    await Promise.all([
      store.load(),
      keys.init(),
      protection.refresh().catch(() => null),
    ]);
  }

  async function createKey() {
    secretKind.value = 'created';
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

  function askDelete() {
    error.value = '';
    force.value = false;
    canForce.value = false;
    showDelete.value = true;
  }

  async function remove() {
    busy.value = true;
    error.value = '';

    try {
      await TuttiService.deleteUser(props.name, force.value);
      await router.push({ name: RouteName.TUTTI_USERS });
    } catch (e) {
      error.value = ProxyAccountHelper.getMessage(e) ?? t('error');
      canForce.value = UserHelper.parseDeleteBlockers(error.value).canForce;
    } finally {
      busy.value = false;
    }
  }

  function onRotated(value: TuttiProxyAccountSecret) {
    secretKind.value = 'rotated';
    secret.value = value;
    rotateId.value = null;
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
    :description="t('usersHint')"
    :state="state"
    :status="
      user
        ? { tone: 'success', label: t(isConfig ? 'config' : 'managed') }
        : undefined
    "
    :tabs="tabs"
    :active-tab="tab"
    @update:active-tab="tab = $event"
    @retry="load"
  >
    <template #back>
      <BackLink
        :to="{ name: RouteName.TUTTI_USERS }"
        :label="t('users')"
      />
    </template>
    <template #actions>
      <WriteGate>
        <NButton
          type="primary"
          :loading="busy"
          :disabled="!user || !store.dynamicEnabled"
          @click="createKey"
          >{{
            user && user.accessKeys?.dynamicActive ? t('newKey') : t('firstKey')
          }}</NButton
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

    <template v-if="user">
      <Callout
        v-if="error && !showDelete"
        tone="danger"
        >{{ error }}</Callout
      >
      <CredentialsTab
        v-if="tab === 'credentials'"
        :user-name="name"
        :storages="storages"
        :dynamic-enabled="store.dynamicEnabled"
        @changed="load"
      />
      <template v-else-if="tab === 'keys'">
        <ProxyAccountKeysTable
          v-if="account"
          :account="account"
          @rotate="rotateId = $event"
        />
        <p
          v-else
          class="user-detail__muted"
        >
          {{ t('keyHint') }}
        </p>
      </template>
      <PermissionsTab
        v-else
        :user="user"
        :groups="groups"
        :wildcard="wildcard"
        @edit="showPolicy = true"
      />

      <WriteGate>
        <FormSection
          class="user-detail__danger"
          :title="t('dangerZone')"
          :description="t('dangerZoneHint')"
        >
          <NButton
            type="error"
            secondary
            :disabled="isConfig"
            @click="askDelete"
            >{{ t('deleteUser') }}</NButton
          >
        </FormSection>
      </WriteGate>
    </template>

    <ConfirmDialog
      :open="showDelete"
      :title="t('deleteUser')"
      :target-name="name"
      :confirm-label="t('deleteUser')"
      :loading="busy"
      require-typing
      danger
      @cancel="showDelete = false"
      @confirm="remove"
    >
      {{ t('confirmDelete', { name }) }}
      <Callout
        v-if="error"
        tone="danger"
        >{{ error }}</Callout
      >
      <NCheckbox
        v-if="canForce"
        v-model:checked="force"
        >{{ t('force') }}</NCheckbox
      >
    </ConfirmDialog>
    <AccessPolicyDrawer
      v-model:show="showPolicy"
      :account="account ?? { user: name }"
      @update:show="!$event && load()"
    />
    <ProxyKeyRotateModal
      :access-key-id="rotateId"
      @close="rotateId = null"
      @rotated="onRotated"
    />
    <ProxySecretModal
      :secret="secret"
      :kind="secretKind"
      @close="closeSecret"
    />
  </DetailLayout>
</template>

<style lang="scss" scoped>
  .user-detail {
    &__muted {
      font-size: var(--tt-text-sm);
      color: var(--tt-text-3);
    }

    &__danger {
      margin-top: var(--tt-space-6);
      padding-top: var(--tt-space-5);
      border-top: 1px solid var(--tt-danger-border);
    }
  }
</style>
