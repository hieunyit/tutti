<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  import { NButton, NInput, NModal } from 'naive-ui';
  import { storeToRefs } from 'pinia';
  import { computed, onMounted, ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nProxies from './i18nProxies';
  import ProxyScopeForm from './ProxyScopeForm.vue';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import {
    Callout,
    ConfirmDialog,
    ErrorBlock,
    LoadingBlock,
  } from '@/components/kit';
  import { useTuttiProxiesStore } from '@/stores/tuttiProxiesStore';
  import {
    DEFAULTS_SCOPE,
    useTuttiProxyFleetConfigStore,
  } from '@/stores/tuttiProxyFleetConfigStore';

  const { t } = useI18n({ messages: i18nProxies });
  const store = useTuttiProxyFleetConfigStore();
  const proxiesStore = useTuttiProxiesStore();
  const {
    config,
    loading,
    loadError,
    saving,
    saveError,
    conflict,
    saved,
    dirty,
    groupNames,
    changes,
    version,
  } = storeToRefs(store);
  const scope = ref(DEFAULTS_SCOPE);
  const adding = ref(false);
  const removing = ref(false);
  const confirming = ref(false);
  const newGroup = ref('');

  const detectedGroups = computed(() =>
    Array.from(
      new Set(
        proxiesStore.proxies
          .map((p) => p.group)
          .filter((g): g is string => !!g && !groupNames.value.includes(g)),
      ),
    ),
  );

  // a group removed (or the draft reloaded) must not leave the editor on a missing scope
  watch(groupNames, (names) => {
    if (scope.value !== DEFAULTS_SCOPE && !names.includes(scope.value))
      scope.value = DEFAULTS_SCOPE;
  });

  const show = (value: unknown) =>
    value === undefined ? t('unset') : String(value);
  const scopeLabel = (value: string) =>
    value === DEFAULTS_SCOPE ? t('clusterDefaults') : value;

  function addGroup() {
    const name = newGroup.value.trim();

    if (!name) return;

    store.addGroup(name);
    scope.value = name;
    newGroup.value = '';
    adding.value = false;
  }

  function removeGroup() {
    store.removeGroup(scope.value);
    removing.value = false;
  }

  async function apply() {
    await store.save();
    confirming.value = false;
    proxiesStore.load();
  }

  onMounted(() => {
    store.load();

    // deep links (/proxies?tab=config) must show the detected groups too
    if (!proxiesStore.loaded) proxiesStore.load();
  });
</script>

<template>
  <div class="config-tab">
    <LoadingBlock v-if="loading && !config" />
    <ErrorBlock
      v-else-if="loadError && !config"
      :description="t('loadError')"
      @retry="store.load(true)"
    />
    <template v-else>
      <p class="config-tab__hint">
        {{ t('version') }}: v{{ version }} · {{ t('fleetConfigHint') }}
      </p>
      <div
        class="config-tab__scopes"
        role="group"
        :aria-label="t('scope')"
      >
        <NButton
          v-for="name in [DEFAULTS_SCOPE, ...groupNames]"
          :key="name"
          size="small"
          :type="scope === name ? 'primary' : 'default'"
          :secondary="scope !== name"
          :aria-pressed="scope === name"
          @click="scope = name"
          >{{ scopeLabel(name) }}</NButton
        >
        <WriteGate>
          <NButton
            size="small"
            quaternary
            @click="adding = true"
            >{{ t('addGroup') }}</NButton
          >
          <NButton
            v-if="scope !== DEFAULTS_SCOPE"
            size="small"
            quaternary
            type="error"
            @click="removing = true"
            >{{ t('removeGroup') }} ({{ scope }})</NButton
          >
        </WriteGate>
      </div>

      <Callout
        v-if="conflict"
        tone="danger"
      >
        {{ t('fleetConflict') }}
        <NButton
          size="small"
          @click="store.load(true)"
          >{{ t('fleetReload') }}</NButton
        >
      </Callout>
      <Callout
        v-if="saveError"
        tone="danger"
        >{{ t('saveError') }}</Callout
      >

      <ProxyScopeForm :scope="scope" />

      <div
        class="config-tab__bar"
        role="region"
        :aria-label="t('fleetConfig')"
      >
        <span
          v-if="dirty"
          class="config-tab__unsaved"
          role="status"
          >{{ t('unsaved') }}</span
        >
        <span
          v-else-if="saved"
          class="config-tab__saved"
          role="status"
          >{{ t('saved') }}</span
        >
        <WriteGate>
          <NButton
            :disabled="!dirty || saving"
            @click="store.reset()"
            >{{ t('discard') }}</NButton
          >
          <NButton
            type="primary"
            :disabled="!dirty"
            :loading="saving"
            @click="confirming = true"
            >{{ t('save') }}</NButton
          >
        </WriteGate>
      </div>
    </template>

    <ConfirmDialog
      :open="confirming"
      :title="t('confirmTitle')"
      :confirm-label="t('confirmApply')"
      :loading="saving"
      @confirm="apply"
      @cancel="confirming = false"
    >
      <p>{{ t('confirmText') }}</p>
      <table
        v-if="changes.length"
        class="config-tab__diff"
      >
        <thead>
          <tr>
            <th scope="col">{{ t('scope') }}</th>
            <th scope="col">{{ t('field') }}</th>
            <th scope="col">{{ t('before') }}</th>
            <th scope="col">{{ t('after') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="change in changes"
            :key="`${change.scope}.${change.path}`"
          >
            <td>{{ scopeLabel(change.scope) }}</td>
            <td>
              <code>{{ change.path }}</code>
            </td>
            <td>{{ show(change.before) }}</td>
            <td>
              <strong>{{ show(change.after) }}</strong>
            </td>
          </tr>
        </tbody>
      </table>
      <p v-else>{{ t('noChanges') }}</p>
    </ConfirmDialog>

    <ConfirmDialog
      :open="removing"
      :title="t('removeGroupTitle')"
      :target-name="scope"
      danger
      :confirm-label="t('removeGroup')"
      @confirm="removeGroup"
      @cancel="removing = false"
    >
      {{ t('removeGroupText') }}
    </ConfirmDialog>

    <NModal
      v-model:show="adding"
      preset="card"
      :title="t('addGroup')"
      class="config-tab__modal"
    >
      <p>{{ t('addGroupHint') }}</p>
      <label
        class="config-tab__label"
        for="fleet-group-name"
        >{{ t('groupName') }}</label
      >
      <NInput
        v-model:value="newGroup"
        :input-props="{ id: 'fleet-group-name' }"
        :placeholder="t('groupNamePlaceholder')"
        @keyup.enter="addGroup"
      />
      <div
        v-if="detectedGroups.length"
        class="config-tab__detected"
      >
        <span>{{ t('detectedGroups') }}</span>
        <NButton
          v-for="group in detectedGroups"
          :key="group"
          size="tiny"
          secondary
          @click="newGroup = group"
          >{{ group }}</NButton
        >
      </div>
      <template #footer>
        <div class="config-tab__modal-actions">
          <NButton @click="adding = false">{{ t('cancel') }}</NButton>
          <NButton
            type="primary"
            :disabled="!newGroup.trim()"
            @click="addGroup"
            >{{ t('addGroupConfirm') }}</NButton
          >
        </div>
      </template>
    </NModal>
  </div>
</template>

<style scoped lang="scss">
  .config-tab {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-4);
    min-width: 0;

    &__hint {
      margin: 0;
      max-width: var(--tt-measure);
      color: var(--tt-text-3);
      font-size: var(--tt-text-sm);
    }

    &__scopes {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      gap: var(--tt-space-2);
    }

    &__bar {
      position: sticky;
      bottom: 0;
      display: flex;
      align-items: center;
      justify-content: flex-end;
      gap: var(--tt-space-2);
      padding: var(--tt-space-3) 0;
      background-color: var(--tt-bg);
      border-top: 1px solid var(--tt-border);
    }

    &__unsaved {
      margin-right: auto;
      color: var(--tt-warning);
      font-size: var(--tt-text-sm);
    }

    &__saved {
      margin-right: auto;
      color: var(--tt-success);
      font-size: var(--tt-text-sm);
    }

    &__diff {
      width: 100%;
      border-collapse: collapse;
      font-size: var(--tt-text-sm);

      th,
      td {
        padding: var(--tt-space-1) var(--tt-space-2);
        text-align: left;
        border-bottom: 1px solid var(--tt-border);
        overflow-wrap: anywhere;
      }
    }

    &__label {
      display: block;
      margin: var(--tt-space-3) 0 var(--tt-space-1);
      font-size: var(--tt-text-sm);
    }

    &__detected {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      gap: var(--tt-space-2);
      margin-top: var(--tt-space-3);
      font-size: var(--tt-text-sm);
    }

    &__modal {
      width: min(460px, 94vw);
    }

    &__modal-actions {
      display: flex;
      justify-content: flex-end;
      gap: var(--tt-space-2);
    }
  }
</style>
