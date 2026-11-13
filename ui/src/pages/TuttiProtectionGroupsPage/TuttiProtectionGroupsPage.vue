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
  import { NButton, NSwitch, type DataTableColumns } from 'naive-ui';
  import { storeToRefs } from 'pinia';
  import { computed, h, onBeforeMount, onBeforeUnmount, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { RouterLink, useRouter } from 'vue-router';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import {
    ConfirmDialog,
    DataTable,
    EMPTY_VALUE,
    EmptyBlock,
    ErrorBlock,
    PageShell,
    StatTile,
    StatusBadge,
    type PageState,
    type StatusTone,
  } from '@/components/kit';
  import i18nProtection from '@/components/tutti/protection/i18nProtectionUi';
  import ProtectionGroupWizard from '@/components/tutti/protection/ProtectionGroupWizard.vue';
  import { useTuttiProtectionStore } from '@/stores/tuttiProtectionStore';
  import { formatDateTime } from '@/utils/helpers/DateTimeHelper';
  import { ProtectionGroupHelper } from '@/utils/helpers/ProtectionGroupHelper';
  import type { ProtectionGroup } from '@/utils/types/protection';
  import { RouteName } from '@/utils/types/router';

  const { t } = useI18n({ messages: i18nProtection });
  const router = useRouter();
  const store = useTuttiProtectionStore();
  const { isLoading, isLoaded, hasError, groups, statuses, automaticFrozen } =
    storeToRefs(store);

  const showCreate = ref(false);
  const showFreezeConfirm = ref(false);
  const isFreezing = ref(false);
  const freezeError = ref(false);

  onBeforeMount(() => store.startPolling());
  onBeforeUnmount(() => store.stopPolling());

  const state = computed<PageState>(() => {
    if (hasError.value) return 'error';

    if (!isLoaded.value) return 'loading';

    return groups.value.length === 0 ? 'empty' : 'ready';
  });

  const switchingCount = computed(
    () => groups.value.filter((g) => !!g.target).length,
  );
  const maxLag = computed(() => {
    const lags = groups.value
      .map((g) => ProtectionGroupHelper.maxLag(statuses.value[g.id]))
      .filter((lag): lag is number => lag !== null);

    return lags.length ? Math.max(...lags) : null;
  });

  function scopeText(group: ProtectionGroup): string {
    const kind = ProtectionGroupHelper.scopeKind(group);

    if (kind === 'storage') return t('scopeStorage');

    if (kind === 'user') return t('scopeUser');

    const count = (group.buckets ?? []).length;

    return count === 1 ? t('scopeBucketsOne') : t('scopeBuckets', { count });
  }

  const columns = computed<DataTableColumns<ProtectionGroup>>(() => [
    {
      title: t('colName'),
      key: 'id',
      render: (g) =>
        h(
          RouterLink,
          {
            to: {
              name: RouteName.TUTTI_PROTECTION_GROUP_DETAIL,
              params: { name: g.id },
            },
            class: 'tt-mono',
          },
          { default: () => g.id },
        ),
    },
    { title: t('colScope'), key: 'scope', render: scopeText },
    {
      title: t('colCopies'),
      key: 'copies',
      render: (g) =>
        t('copiesValue', {
          healthy: statuses.value[g.id]?.healthyCopies ?? EMPTY_VALUE,
          wanted: ProtectionGroupHelper.wantedCopies(g),
        }),
    },
    {
      title: t('colLag'),
      key: 'lag',
      render: (g) =>
        ProtectionGroupHelper.lagText(
          ProtectionGroupHelper.maxLag(statuses.value[g.id]),
        ),
    },
    {
      title: t('colChange'),
      key: 'change',
      render: (g) =>
        formatDateTime(ProtectionGroupHelper.historyNewestFirst(g)[0]?.at),
    },
    {
      title: t('colState'),
      key: 'state',
      render: (g) => {
        const display = ProtectionGroupHelper.displayState(g);

        return h(StatusBadge, {
          tone: ProtectionGroupHelper.stateVariant(display) as StatusTone,
          label: t(`state.${display}`),
        });
      },
    },
  ]);

  async function confirmToggleFreeze() {
    isFreezing.value = true;
    freezeError.value = false;

    const err = await store.setAutomaticFreeze(!automaticFrozen.value);

    isFreezing.value = false;
    showFreezeConfirm.value = false;
    freezeError.value = err !== null;
  }

  function openGroup(id: string) {
    router.push({
      name: RouteName.TUTTI_PROTECTION_GROUP_DETAIL,
      params: { name: id },
    });
  }
</script>

<template>
  <PageShell
    :title="t('title')"
    :description="t('pageDescription')"
    :state="state"
    :error-text="t('errorText')"
    @retry="store.refresh()"
  >
    <template #actions>
      <label
        v-if="automaticFrozen !== null"
        class="pg-list__freeze"
      >
        <span>{{ t('freezeSwitch') }}</span>
        <WriteGate>
          <NSwitch
            :value="!automaticFrozen"
            :loading="isFreezing"
            @update:value="showFreezeConfirm = true"
          />
        </WriteGate>
        <span class="pg-list__freeze-state">
          {{ automaticFrozen ? t('freezeSwitchOff') : t('freezeSwitchOn') }}
        </span>
      </label>
      <NButton
        :loading="isLoading"
        @click="store.refresh()"
      >
        {{ t('refresh') }}
      </NButton>
      <WriteGate v-if="!hasError">
        <NButton
          type="primary"
          @click="showCreate = true"
        >
          {{ t('create') }}
        </NButton>
      </WriteGate>
    </template>

    <template #empty>
      <EmptyBlock
        :title="t('emptyTitle')"
        :description="t('emptyText')"
      >
        <WriteGate>
          <NButton
            type="primary"
            @click="showCreate = true"
          >
            {{ t('create') }}
          </NButton>
        </WriteGate>
      </EmptyBlock>
    </template>

    <div class="pg-list">
      <ErrorBlock
        v-if="freezeError"
        :title="t('freezeErrorTitle')"
        :description="t('freezeFailed')"
        @retry="showFreezeConfirm = true"
      />
      <section class="pg-list__tiles">
        <StatTile
          :label="t('kpiGroups')"
          :value="groups.length"
          :hint="t('kpiGroupsSub')"
        />
        <StatTile
          :label="t('kpiSwitching')"
          :value="switchingCount"
          :hint="t('kpiSwitchingSub')"
          :tone="switchingCount > 0 ? 'warning' : 'neutral'"
        />
        <StatTile
          :label="t('kpiMaxLag')"
          :value="ProtectionGroupHelper.lagText(maxLag)"
          :hint="t('kpiMaxLagSub')"
          :tone="maxLag ? 'warning' : 'neutral'"
        />
      </section>
      <DataTable
        :columns="columns"
        :rows="groups"
        :loading="isLoading && !isLoaded"
        :row-key="(g: ProtectionGroup) => g.id"
        @retry="store.refresh()"
      />
    </div>
  </PageShell>

  <ProtectionGroupWizard
    v-model:show="showCreate"
    @saved="openGroup"
  />

  <ConfirmDialog
    :open="showFreezeConfirm"
    :title="automaticFrozen ? t('freezeDisableTitle') : t('freezeEnableTitle')"
    :danger="!automaticFrozen"
    :loading="isFreezing"
    :confirm-label="
      automaticFrozen ? t('freezeDisableAction') : t('freezeEnableAction')
    "
    @confirm="confirmToggleFreeze"
    @cancel="showFreezeConfirm = false"
  >
    {{ automaticFrozen ? t('freezeDisableText') : t('freezeEnableText') }}
  </ConfirmDialog>
</template>

<style lang="scss" scoped>
  .pg-list {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-4);

    &__tiles {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
      gap: var(--tt-space-4);
    }

    &__freeze {
      display: inline-flex;
      align-items: center;
      gap: var(--tt-space-2);
      font-size: var(--tt-text-sm);
      color: var(--tt-text-2);
    }

    &__freeze-state {
      font-weight: var(--tt-weight-medium);
      color: var(--tt-text);
    }
  }
</style>
