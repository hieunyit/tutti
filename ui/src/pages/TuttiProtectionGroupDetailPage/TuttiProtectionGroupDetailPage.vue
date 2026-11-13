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
  import { NButton } from 'naive-ui';
  import { storeToRefs } from 'pinia';
  import { computed, onBeforeMount, onBeforeUnmount, ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { useRoute, useRouter } from 'vue-router';
  import ActivityTab from './ActivityTab.vue';
  import GroupActions from './GroupActions.vue';
  import MembersTab from './MembersTab.vue';
  import SettingsTab from './SettingsTab.vue';
  import WritersTab from './WritersTab.vue';
  import {
    BackLink,
    Callout,
    DetailLayout,
    EmptyBlock,
    useQueryTab,
    type PageState,
    type StatusTone,
  } from '@/components/kit';
  import i18nProtection from '@/components/tutti/protection/i18nProtectionUi';
  import ProtectionGroupDialog from '@/components/tutti/protection/ProtectionGroupDialog.vue';
  import { useTuttiProtectionStore } from '@/stores/tuttiProtectionStore';
  import { useProtectionGroupActions } from '@/utils/composables/useProtectionGroupActions';
  import { ProtectionGroupHelper } from '@/utils/helpers/ProtectionGroupHelper';
  import { RouteName } from '@/utils/types/router';

  const props = defineProps<{ name?: string }>();

  const { t } = useI18n({ messages: i18nProtection });
  const route = useRoute();
  const router = useRouter();
  const store = useTuttiProtectionStore();
  const { detail, detailLoading, detailMissing, detailError, writers } =
    storeToRefs(store);
  const { writersError } = storeToRefs(store);

  const groupId = computed(() => props.name || String(route.params.name ?? ''));
  const group = computed(() => detail.value?.group ?? null);
  const actions = useProtectionGroupActions(groupId);
  const tab = useQueryTab('tab', 'members');
  const showEdit = ref(false);

  function startPolling() {
    if (!groupId.value) return;

    store.startPolling(() =>
      store.loadDetail(groupId.value, store.detail !== null),
    );
  }

  onBeforeMount(() => {
    store.resetDetail();
    startPolling();
  });
  watch(groupId, () => {
    store.resetDetail();
    startPolling();
  });
  onBeforeUnmount(() => {
    store.stopPolling();
    store.resetDetail();
  });

  const state = computed<PageState>(() => {
    if (detailMissing.value) return 'empty';

    if (detailError.value && !group.value) return 'error';

    return group.value ? 'ready' : 'loading';
  });

  const displayState = computed(() =>
    group.value ? ProtectionGroupHelper.displayState(group.value) : 'unknown',
  );
  const status = computed(() =>
    group.value
      ? {
          tone: ProtectionGroupHelper.stateVariant(
            displayState.value,
          ) as StatusTone,
          label: t(`state.${displayState.value}`),
        }
      : undefined,
  );
  const description = computed(() => {
    const g = group.value;

    if (!g) return undefined;

    const parts = [
      g.primary ? `${t('rolePrimary')}: ${g.primary}` : '',
      t('generation', { value: g.generation ?? 0 }),
      g.mode === 'auto' ? t('modeAuto') : t('modeManual'),
    ];

    return parts.filter(Boolean).join(' · ');
  });
  const tabs = computed(() => [
    { key: 'members', label: t('tabMembers') },
    { key: 'writers', label: t('tabWriters') },
    { key: 'activity', label: t('tabActivity') },
    { key: 'settings', label: t('tabSettings') },
  ]);

  function lagOf(storage: string) {
    const rows = group.value
      ? ProtectionGroupHelper.memberRows(group.value, detail.value ?? undefined)
      : [];

    return rows.find((r) => r.storage === storage)?.lag ?? 0;
  }

  function backToList() {
    router.push({ name: RouteName.TUTTI_PROTECTION_GROUPS });
  }
</script>

<template>
  <DetailLayout
    :title="group?.id ?? groupId"
    :description="description"
    :state="state"
    :status="status"
    :tabs="group ? tabs : undefined"
    :active-tab="tab"
    :error-text="t('errorText')"
    @update:active-tab="tab = $event"
    @retry="store.loadDetail(groupId)"
  >
    <template #back>
      <BackLink
        :to="{ name: RouteName.TUTTI_PROTECTION_GROUPS }"
        :label="t('back')"
      />
    </template>

    <template #actions>
      <template v-if="group">
        <NButton
          :loading="detailLoading"
          @click="store.loadDetail(groupId)"
        >
          {{ t('refreshDetail') }}
        </NButton>
        <GroupActions
          :group="group"
          :lag-of="lagOf"
          :actions="actions"
        />
      </template>
    </template>

    <template #empty>
      <EmptyBlock
        :title="t('notFound')"
        :description="t('notFoundText')"
      >
        <NButton
          type="primary"
          @click="backToList"
        >
          {{ t('back') }}
        </NButton>
      </EmptyBlock>
    </template>

    <template v-if="group">
      <Callout
        v-if="group.frozen"
        tone="warning"
        :title="t('frozenBannerTitle')"
      >
        {{ t('frozenBanner') }}
      </Callout>
      <Callout
        v-if="(group.quarantined ?? []).length > 0"
        tone="warning"
      >
        {{
          t('quarantineBanner', {
            members: (group.quarantined ?? []).join(', '),
          })
        }}
      </Callout>
      <Callout
        v-if="group.lastError"
        tone="danger"
        :title="t('lastErrorTitle')"
      >
        {{ group.lastError }} · {{ t('lastErrorHint') }}
      </Callout>

      <MembersTab
        v-if="tab === 'members'"
        :group="group"
        :detail="detail"
      />
      <WritersTab
        v-else-if="tab === 'writers'"
        :group="group"
        :writers="writers"
        :writers-error="writersError"
        :actions="actions"
        @retry="store.loadWriters(groupId)"
      />
      <ActivityTab
        v-else-if="tab === 'activity'"
        :group="group"
      />
      <SettingsTab
        v-else
        :group="group"
        :detail="detail"
        :actions="actions"
        @edit="showEdit = true"
        @removed="backToList"
      />

      <ProtectionGroupDialog
        v-model:show="showEdit"
        :group="group"
      />
    </template>
  </DetailLayout>
</template>
