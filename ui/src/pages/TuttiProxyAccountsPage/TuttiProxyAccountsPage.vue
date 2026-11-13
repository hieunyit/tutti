<!--
  - Copyright © 2026 hieunyit
  -
  -  Licensed under the GNU Affero General Public License, Version 3.0 (the "License");
  -  you may not use this file except in compliance with the License.
  -  You may obtain a copy of the License at
  -
  -  https://www.gnu.org/licenses/agpl-3.0.html
  -
  -  Unless required by applicable law or agreed to in writing, software
  -  distributed under the License is distributed on an "AS IS" BASIS,
  -  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  -  See the License for the specific language governing permissions and
  -  limitations under the License.
  -->

<script setup lang="ts">
  import { storeToRefs } from 'pinia';
  import { computed, onBeforeMount, onBeforeUnmount, ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NButton, NTooltip } from 'naive-ui';
  import i18nAccounts from '@/components/tutti/accounts/i18nAccounts';
  import ProxyAccountsTile from '@/components/tutti/accounts/ProxyAccountsTile.vue';
  import { Callout, EmptyBlock, PageShell } from '@/components/kit';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import { useTuttiAccessPolicyStore } from '@/stores/tuttiAccessPolicyStore';

  const { t } = useI18n({ messages: i18nAccounts });
  const store = useTuttiAccessPolicyStore();
  const { hasError, isLoading, dynamicAvailable, dynamicReason, accounts } =
    storeToRefs(store);

  const accountsTile = ref<InstanceType<typeof ProxyAccountsTile> | null>(null);
  // the store has no "loaded" flag: loading is the first request in flight
  const loaded = ref(false);
  const state = computed(() => {
    if (hasError.value) return 'error' as const;

    return loaded.value || accounts.value.length > 0
      ? ('ready' as const)
      : ('loading' as const);
  });

  // the tile is not mounted while the first load runs: queue the open
  let createQueued = false;

  function openCreate() {
    if (accountsTile.value) accountsTile.value.openCreate();
    else createQueued = true;
  }

  watch(accountsTile, (tile) => {
    if (tile && createQueued) {
      createQueued = false;
      tile.openCreate();
    }
  });

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
    :description="t('pageDescription')"
    :state="state"
    :error-text="t('errorText')"
    @retry="reload"
  >
    <template #actions>
      <NButton
        :loading="isLoading"
        @click="reload"
        >{{ t('refresh') }}</NButton
      >
      <WriteGate v-if="!hasError">
        <NTooltip :disabled="dynamicAvailable">
          <template #trigger>
            <span class="proxy-accounts-create">
              <NButton
                type="primary"
                :disabled="!dynamicAvailable"
                @click="openCreate"
                >{{ t('createAccount') }}</NButton
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

    <EmptyBlock
      v-if="!dynamicAvailable && accounts.length === 0"
      :title="t('unavailableTitle')"
      :description="t('unavailableText')"
    />
    <template v-else>
      <Callout
        v-if="!dynamicAvailable"
        tone="warning"
        :title="t('unavailableTitle')"
        >{{ t('unavailableText') }}</Callout
      >
      <ProxyAccountsTile ref="accountsTile" />
    </template>
  </PageShell>
</template>

<style lang="scss" scoped>
  .proxy-accounts-create {
    display: inline-flex;
  }
</style>
