<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  import { storeToRefs } from 'pinia';
  import { computed, onBeforeMount, onBeforeUnmount, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { useRoute } from 'vue-router';
  import { NButton, NTooltip } from 'naive-ui';
  import i18nAccounts from '@/components/tutti/accounts/i18nAccounts';
  import ProxyAccountCreateModal from '@/components/tutti/accounts/ProxyAccountCreateModal.vue';
  import ProxyAccountKeysTable from '@/components/tutti/accounts/ProxyAccountKeysTable.vue';
  import ProxyKeyRotateModal from '@/components/tutti/accounts/ProxyKeyRotateModal.vue';
  import ProxySecretModal from '@/components/tutti/accounts/ProxySecretModal.vue';
  import {
    BackLink,
    Callout,
    EmptyBlock,
    PageShell,
    TableToolbar,
  } from '@/components/kit';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import { useTuttiAccessPolicyStore } from '@/stores/tuttiAccessPolicyStore';
  import { ProxyAccountHelper } from '@/utils/helpers/ProxyAccountHelper';
  import { RouteName } from '@/utils/types/router';
  import type { TuttiProxyAccountSecret } from '@/utils/types/tutti';

  const { t } = useI18n({ messages: i18nAccounts });
  const store = useTuttiAccessPolicyStore();
  const route = useRoute();
  const { hasError, dynamicAvailable, dynamicReason, accounts, proxyAddress } =
    storeToRefs(store);

  /** `?user=alice` (link from the Users pages) pre-fills the search */
  const forUser = computed(() => {
    const value = route?.query?.user;

    return typeof value === 'string' ? value : '';
  });
  const query = ref(forUser.value);
  // the store has no "loaded" flag: loading is the first request in flight
  const loaded = ref(false);
  const showCreate = ref(false);
  const rotateKeyId = ref<string | null>(null);
  /** one-time secret being displayed; cleared on close, never persisted */
  const shownSecret = ref<TuttiProxyAccountSecret | null>(null);
  const shownKind = ref<'created' | 'rotated'>('created');

  const hasKeys = computed(() =>
    accounts.value.some((account) => (account.accessKeyIds ?? []).length > 0),
  );
  const visible = computed(() =>
    ProxyAccountHelper.filterAccounts(accounts.value, query.value),
  );
  const state = computed(() => {
    if (hasError.value) return 'error' as const;

    if (!loaded.value && accounts.value.length === 0) return 'loading' as const;

    return hasKeys.value ? ('ready' as const) : ('empty' as const);
  });

  function onCreated(secret: TuttiProxyAccountSecret) {
    shownKind.value = 'created';
    shownSecret.value = secret;
  }

  function onRotated(secret: TuttiProxyAccountSecret) {
    rotateKeyId.value = null;
    shownKind.value = 'rotated';
    shownSecret.value = secret;
  }

  async function reload() {
    await store.init();
    loaded.value = true;
  }

  onBeforeMount(reload);
  onBeforeUnmount(() => store.$reset());
</script>

<template>
  <PageShell
    :title="t('title')"
    :state="state"
    :error-text="t('errorText')"
    @retry="reload"
  >
    <template
      v-if="forUser"
      #back
    >
      <BackLink
        :to="{ name: RouteName.TUTTI_USER_DETAIL, params: { name: forUser } }"
        :label="t('backToUser', { user: forUser })"
      />
    </template>
    <template
      v-if="state === 'ready'"
      #actions
    >
      <WriteGate>
        <NTooltip :disabled="dynamicAvailable">
          <template #trigger>
            <span class="access-keys__create">
              <NButton
                type="primary"
                :disabled="!dynamicAvailable"
                @click="showCreate = true"
                >{{ t('createKey') }}</NButton
              >
            </span>
          </template>
          {{
            t('unavailableReason', {
              reason: dynamicReason || t('notConfigured'),
            })
          }}
        </NTooltip>
      </WriteGate>
    </template>
    <template #empty>
      <EmptyBlock
        v-if="!dynamicAvailable"
        :title="t('unavailableTitle')"
        :description="t('unavailableText')"
      />
      <EmptyBlock
        v-else
        :title="t('emptyTitle')"
        :description="t('emptyText')"
      >
        <WriteGate>
          <NButton
            type="primary"
            @click="showCreate = true"
            >{{ t('createKey') }}</NButton
          >
        </WriteGate>
      </EmptyBlock>
    </template>

    <div class="access-keys">
      <Callout
        v-if="!dynamicAvailable"
        tone="warning"
        :title="t('unavailableTitle')"
        >{{ t('unavailableText') }}</Callout
      >
      <TableToolbar
        v-model:search="query"
        :search-label="t('searchLabel')"
        :search-placeholder="t('searchLabel')"
        @clear="query = ''"
      />
      <ProxyAccountKeysTable
        v-if="visible.length > 0"
        :accounts="visible"
        @rotate="rotateKeyId = $event"
      />
      <EmptyBlock
        v-else
        :title="t('noResultsTitle')"
      >
        <NButton @click="query = ''">{{ t('clearFilters') }}</NButton>
      </EmptyBlock>
    </div>

    <ProxyAccountCreateModal
      v-model:show="showCreate"
      :users="accounts.map((a) => a.user)"
      @created="onCreated"
    />
    <ProxyKeyRotateModal
      :access-key-id="rotateKeyId"
      @close="rotateKeyId = null"
      @rotated="onRotated"
    />
    <ProxySecretModal
      :secret="shownSecret"
      :kind="shownKind"
      :endpoint="proxyAddress"
      @close="shownSecret = null"
    />
  </PageShell>
</template>

<style lang="scss" scoped>
  .access-keys {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-4);
    min-width: 0;

    &__create {
      display: inline-flex;
    }
  }
</style>
