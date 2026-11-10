<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  import { computed, onMounted, onBeforeUnmount, ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NButton, NInput, NInputNumber, NSelect } from 'naive-ui';
  import PageHeader from '@/components/layout/PageHeader.vue';
  import SectionCard from '@/components/ui/SectionCard.vue';
  import StatusTag from '@/components/ui/StatusTag.vue';
  import Callout from '@/components/ui/Callout.vue';
  import EmptyState from '@/components/ui/EmptyState.vue';
  import TabBar, { type TabItem } from '@/components/tutti/common/TabBar.vue';
  import messages from '@/components/tutti/users/i18nManagement';
  import { TuttiService } from '@/services/TuttiService';
  import { ProxiesHelper } from '@/utils/helpers/ProxiesHelper';
  import { DashboardHelper } from '@/utils/helpers/DashboardHelper';
  import { formatRelative } from '@/utils/helpers/DateTimeHelper';
  import type {
    TuttiInt64,
    TuttiProxyFleetConfig,
    TuttiProxySettingsOverride,
    TuttiProxyStatus,
  } from '@/utils/types/tutti';

  const { t } = useI18n({ messages });
  const proxies = ref<TuttiProxyStatus[]>([]);
  const configVersion = ref<TuttiInt64>(0);
  const fleetConfig = ref<TuttiProxyFleetConfig | null>(null);
  const error = ref(false);
  const busy = ref(false);
  const savingFleet = ref(false);
  const savedFleet = ref(false);
  const fleetConflict = ref(false);
  const editingScope = ref<string>('__defaults__');
  const newGroupName = ref('');
  const defaultsDraft = ref<TuttiProxySettingsOverride>({});
  const groupsDraft = ref<Record<string, TuttiProxySettingsOverride>>({});
  const now = ref(Date.now());
  let timer: ReturnType<typeof setInterval> | undefined;
  let controller: AbortController | null = null;

  const drift = computed(() =>
    ProxiesHelper.settingsDrift(proxies.value, now.value),
  );
  const outliers = computed(() => ProxiesHelper.outliers(drift.value));

  const groupNames = computed(() => Object.keys(groupsDraft.value).sort());

  const activeDraft = computed<TuttiProxySettingsOverride>(() => {
    if (editingScope.value === '__defaults__') {
      return defaultsDraft.value;
    }

    return groupsDraft.value[editingScope.value] ?? {};
  });

  // the draft of the group being edited must exist before the form binds to it (kept out of the computed)
  watch(
    [editingScope, groupsDraft],
    () => {
      if (
        editingScope.value !== '__defaults__' &&
        !groupsDraft.value[editingScope.value]
      ) {
        groupsDraft.value[editingScope.value] = {};
      }
    },
    { flush: 'sync', immediate: true },
  );

  function cloneOverride(
    src?: TuttiProxySettingsOverride,
  ): TuttiProxySettingsOverride {
    return src
      ? (JSON.parse(JSON.stringify(src)) as TuttiProxySettingsOverride)
      : {};
  }

  function hydrateFleetDraft(fc: TuttiProxyFleetConfig) {
    fleetConfig.value = fc;
    defaultsDraft.value = cloneOverride(fc.defaults);
    const nextGroups: Record<string, TuttiProxySettingsOverride> = {};

    Object.entries(fc.groups ?? {}).forEach(([group, ov]) => {
      nextGroups[group] = cloneOverride(ov);
    });
    groupsDraft.value = nextGroups;

    if (
      editingScope.value !== '__defaults__' &&
      !nextGroups[editingScope.value]
    ) {
      editingScope.value = '__defaults__';
    }
  }

  async function reloadFleetConfig() {
    fleetConflict.value = false;
    await loadFleetConfig();
  }

  async function loadFleetConfig() {
    try {
      const fc = await TuttiService.getProxyFleetConfig();

      hydrateFleetDraft(fc);
    } catch {
      // ignore if endpoint unavailable until next refresh
    }
  }

  async function load() {
    if (controller) return;

    const request = new AbortController();

    controller = request;
    busy.value = true;

    try {
      const result = await TuttiService.getProxies(request.signal);

      if (controller !== request) return;

      proxies.value = result.proxies ?? [];
      configVersion.value = result.configVersion ?? 0;
      error.value = false;
      now.value = Date.now();
    } catch {
      if (controller === request) error.value = true;
    } finally {
      if (controller === request) {
        controller = null;
        busy.value = false;
      }
    }
  }

  function addGroupOverride() {
    const trimmed = newGroupName.value.trim();

    if (!trimmed) return;

    if (!groupsDraft.value[trimmed]) {
      groupsDraft.value[trimmed] = {};
    }

    editingScope.value = trimmed;
    newGroupName.value = '';
  }

  function removeGroupOverride(group: string) {
    const next = { ...groupsDraft.value };

    delete next[group];
    groupsDraft.value = next;

    if (editingScope.value === group) {
      editingScope.value = '__defaults__';
    }
  }

  const scopeTabs = computed<TabItem[]>(() => [
    { label: t('clusterDefaults'), value: '__defaults__' },
    ...groupNames.value.map((grp) => ({ label: grp, value: grp })),
  ]);

  const boolOptions = computed(() => [
    { label: t('inherit'), value: '' },
    { label: t('on'), value: 'true' },
    { label: t('off'), value: 'false' },
  ]);

  const quorumFailureOptions = computed(() => [
    { label: t('inherit'), value: '' },
    { label: 'fail', value: 'fail' },
    { label: 'accept', value: 'accept' },
  ]);

  const eventErrorOptions = computed(() => [
    { label: t('inherit'), value: '' },
    { label: 'fail', value: 'fail' },
    { label: 'log', value: 'log' },
  ]);

  function setBoolField(
    target: Record<string, unknown>,
    key: string,
    val: string | null,
  ) {
    if (!val) {
      delete target[key];
    } else {
      target[key] = val === 'true';
    }
  }

  function setStrField(
    target: Record<string, unknown>,
    key: string,
    val: string | null,
  ) {
    const trimmed = (val ?? '').trim();

    if (!trimmed) {
      delete target[key];
    } else {
      target[key] = trimmed;
    }
  }

  function setNumVal(
    target: Record<string, unknown>,
    key: string,
    val: number | null,
  ) {
    if (val === null || val === undefined || !Number.isFinite(val)) {
      delete target[key];
    } else {
      target[key] = val;
    }
  }

  function ensureReplication(ov: TuttiProxySettingsOverride) {
    if (!ov.replication) ov.replication = {};

    return ov.replication;
  }

  function ensureSync(ov: TuttiProxySettingsOverride) {
    const repl = ensureReplication(ov);

    if (!repl.sync) repl.sync = {};

    return repl.sync;
  }

  function ensureHealth(ov: TuttiProxySettingsOverride) {
    if (!ov.health) ov.health = {};

    return ov.health;
  }

  function ensureFailover(ov: TuttiProxySettingsOverride) {
    if (!ov.failover) ov.failover = {};

    return ov.failover;
  }

  async function saveFleetConfig() {
    savingFleet.value = true;
    savedFleet.value = false;
    error.value = false;

    try {
      const updated = await TuttiService.setProxyFleetConfig({
        defaults: defaultsDraft.value,
        groups: groupsDraft.value,
        // optimistic concurrency: only the version this editor loaded may be overwritten
        expectedVersion: fleetConfig.value?.version ?? 0,
      });

      fleetConflict.value = false;
      hydrateFleetDraft(updated);
      configVersion.value = updated.version ?? configVersion.value;
      savedFleet.value = true;
      await load();
    } catch (e) {
      if (ProxiesHelper.isFleetConflict(e)) {
        fleetConflict.value = true;
      } else {
        error.value = true;
      }
    } finally {
      savingFleet.value = false;
    }
  }

  onMounted(() => {
    load();
    loadFleetConfig();
    timer = setInterval(load, 2000);
  });
  onBeforeUnmount(() => {
    clearInterval(timer);
    controller?.abort();
    controller = null;
  });
  const number = (value?: number) =>
    value === undefined
      ? '–'
      : value.toLocaleString(undefined, { maximumFractionDigits: 2 });
</script>

<template>
  <div class="phase-d">
    <PageHeader
      :title="t('proxies')"
      :description="t('proxiesHint')"
      ><template #actions
        ><NButton
          :loading="busy"
          @click="
            load();
            loadFleetConfig();
          "
          >{{ t('refresh') }}</NButton
        ></template
      ></PageHeader
    ><Callout
      v-if="drift.length"
      tone="warning"
      >{{ t('drift') }}
      <div
        v-for="item in drift"
        :key="item.field"
        class="phase-d__muted"
      >
        {{
          item.groups
            .map((group) => `${group.proxies.join(', ')}: ${group.value}`)
            .join(' · ')
        }}
      </div></Callout
    ><Callout
      v-if="error"
      tone="danger"
      >{{ t('error') }}</Callout
    ><SectionCard
      :title="t('proxies')"
      flush
      ><table
        v-if="proxies.length"
        class="phase-d__table"
      >
        <thead>
          <tr>
            <th>{{ t('proxies') }}</th>
            <th>{{ t('group') }}</th>
            <th>{{ t('configSync') }}</th>
            <th>{{ t('status') }}</th>
            <th>{{ t('uptime') }}</th>
            <th>{{ t('requests') }}</th>
            <th>{{ t('p95') }}</th>
            <th>{{ t('errors') }}</th>
            <th>{{ t('storages') }}</th>
            <th>{{ t('failover') }}</th>
            <th>{{ t('sync') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="proxy in proxies"
            :key="proxy.appId"
          >
            <td>
              <strong>{{ proxy.appId }}</strong>
              <div class="phase-d__muted">{{ proxy.address }}</div>
              <StatusTag
                v-if="outliers.has(proxy.appId)"
                variant="warning"
                >{{ t('settings') }}</StatusTag
              >
            </td>
            <td>
              <StatusTag variant="neutral">{{
                ProxiesHelper.effectiveGroup(proxy)
              }}</StatusTag>
            </td>
            <td>
              <StatusTag
                v-if="
                  ProxiesHelper.configSyncStatus(proxy, configVersion).state ===
                  'applied'
                "
                variant="success"
                >{{
                  t('appliedVersion', {
                    version: ProxiesHelper.configSyncStatus(
                      proxy,
                      configVersion,
                    ).appliedVersion,
                  })
                }}</StatusTag
              >
              <StatusTag
                v-else-if="
                  ProxiesHelper.configSyncStatus(proxy, configVersion).state ===
                  'applying'
                "
                variant="warning"
                >{{
                  t('applyingVersion', {
                    version: ProxiesHelper.configSyncStatus(
                      proxy,
                      configVersion,
                    ).targetVersion,
                  })
                }}</StatusTag
              >
              <StatusTag
                v-else
                variant="neutral"
                >{{ t('startupConfig') }}</StatusTag
              >
              <div v-if="proxy.configError">
                <StatusTag variant="danger">{{
                  t('configRejected')
                }}</StatusTag>
                <div class="phase-d__muted">{{ proxy.configError }}</div>
              </div>
              <div v-if="proxy.pendingRestart">
                <StatusTag variant="warning">{{
                  t('pendingRestart')
                }}</StatusTag>
                <div
                  v-if="proxy.pendingRestartReasons?.length"
                  class="phase-d__muted"
                >
                  {{ proxy.pendingRestartReasons.join(', ') }}
                </div>
              </div>
            </td>
            <td>
              <StatusTag
                :variant="
                  DashboardHelper.isProxyReporting(proxy, now)
                    ? 'success'
                    : 'warning'
                "
                >{{
                  t(
                    DashboardHelper.isProxyReporting(proxy, now)
                      ? 'reporting'
                      : 'stale',
                  )
                }}</StatusTag
              >
              <div class="phase-d__muted">
                {{ formatRelative(proxy.updatedAt, undefined, now) }}
              </div>
            </td>
            <td>{{ ProxiesHelper.uptime(proxy.startedAt, now) ?? '–' }}</td>
            <td>{{ number(proxy.rates?.requestsPerSec) }}</td>
            <td>{{ number(proxy.rates?.latencyP95Ms) }} ms</td>
            <td>{{ number(proxy.rates?.status5xxPerSec) }}</td>
            <td>
              {{ ProxiesHelper.storageAvailability(proxy).available }} /
              {{ ProxiesHelper.storageAvailability(proxy).total }}
            </td>
            <td>
              <StatusTag
                :variant="
                  proxy.settings?.failover?.enabled ? 'success' : 'neutral'
                "
                >{{
                  t(proxy.settings?.failover?.enabled ? 'on' : 'off')
                }}</StatusTag
              >
              <div class="phase-d__muted">
                {{ proxy.settings?.failover?.after }}
              </div>
            </td>
            <td>
              <StatusTag
                :variant="
                  proxy.settings?.replication?.sync?.enabled
                    ? 'success'
                    : 'neutral'
                "
                >{{
                  t(proxy.settings?.replication?.sync?.enabled ? 'on' : 'off')
                }}</StatusTag
              >
              <div
                v-if="proxy.settings?.replication?.sync?.enabled"
                class="phase-d__muted"
              >
                {{
                  t('quorum', {
                    value:
                      proxy.settings.replication.sync.writeQuorum || t('all'),
                  })
                }}
                · {{ proxy.settings.replication.sync.timeout }}
              </div>
            </td>
          </tr>
        </tbody>
      </table>
      <EmptyState
        v-else-if="!busy"
        :text="t('noProxies')" /></SectionCard
    ><SectionCard
      :title="t('fleetConfig')"
      :subtitle="`${t('version')}: v${fleetConfig?.version ?? configVersion ?? 0} · ${t('fleetConfigHint')}`"
    >
      <div class="phase-d__tab-row">
        <TabBar
          v-model="editingScope"
          :tabs="scopeTabs"
          :label="t('fleetConfig')"
        />
        <div class="phase-d__group-actions">
          <NInput
            v-model:value="newGroupName"
            size="small"
            :placeholder="t('groupName')"
            style="width: 180px"
            @keyup.enter="addGroupOverride"
          />
          <NButton
            size="small"
            secondary
            @click="addGroupOverride"
            >{{ t('addGroup') }}</NButton
          >
          <NButton
            v-if="editingScope !== '__defaults__'"
            size="small"
            type="error"
            secondary
            @click="removeGroupOverride(editingScope)"
            >{{ t('remove') }} ({{ editingScope }})</NButton
          >
        </div>
      </div>

      <div class="phase-d__grid">
        <div class="phase-d__field">
          <span class="phase-d__label">{{ t('sync') }}</span>
          <NSelect
            :value="
              activeDraft.replication?.sync?.enabled === undefined
                ? ''
                : String(activeDraft.replication.sync.enabled)
            "
            :options="boolOptions"
            @update:value="
              (val) =>
                setBoolField(
                  ensureSync(activeDraft) as Record<string, unknown>,
                  'enabled',
                  val,
                )
            "
          />
        </div>

        <div class="phase-d__field">
          <span class="phase-d__label">{{ t('writeQuorum') }}</span>
          <NInputNumber
            :value="activeDraft.replication?.sync?.writeQuorum ?? null"
            :min="0"
            :placeholder="t('inherit')"
            clearable
            style="width: 100%"
            @update:value="
              (val) =>
                setNumVal(
                  ensureSync(activeDraft) as Record<string, unknown>,
                  'writeQuorum',
                  val,
                )
            "
          />
        </div>

        <div class="phase-d__field">
          <span class="phase-d__label">{{ t('syncTimeout') }}</span>
          <NInput
            :value="activeDraft.replication?.sync?.timeout ?? ''"
            placeholder="30s"
            clearable
            @update:value="
              (val) =>
                setStrField(
                  ensureSync(activeDraft) as Record<string, unknown>,
                  'timeout',
                  val,
                )
            "
          />
        </div>

        <div class="phase-d__field">
          <span class="phase-d__label">{{ t('onQuorumFailure') }}</span>
          <NSelect
            :value="activeDraft.replication?.sync?.onQuorumFailure ?? ''"
            :options="quorumFailureOptions"
            @update:value="
              (val) =>
                setStrField(
                  ensureSync(activeDraft) as Record<string, unknown>,
                  'onQuorumFailure',
                  val,
                )
            "
          />
        </div>

        <div class="phase-d__field">
          <span class="phase-d__label">{{ t('onEventError') }}</span>
          <NSelect
            :value="activeDraft.replication?.onEventError ?? ''"
            :options="eventErrorOptions"
            @update:value="
              (val) =>
                setStrField(
                  ensureReplication(activeDraft) as Record<string, unknown>,
                  'onEventError',
                  val,
                )
            "
          />
        </div>

        <div class="phase-d__field">
          <span class="phase-d__label">{{ t('failureThreshold') }}</span>
          <NInputNumber
            :value="activeDraft.health?.failureThreshold ?? null"
            :min="1"
            :placeholder="t('inherit')"
            clearable
            style="width: 100%"
            @update:value="
              (val) =>
                setNumVal(
                  ensureHealth(activeDraft) as Record<string, unknown>,
                  'failureThreshold',
                  val,
                )
            "
          />
        </div>

        <div class="phase-d__field">
          <span class="phase-d__label">{{ t('openDuration') }}</span>
          <NInput
            :value="activeDraft.health?.openDuration ?? ''"
            placeholder="30s"
            clearable
            @update:value="
              (val) =>
                setStrField(
                  ensureHealth(activeDraft) as Record<string, unknown>,
                  'openDuration',
                  val,
                )
            "
          />
        </div>

        <div class="phase-d__field">
          <span class="phase-d__label">{{ t('failover') }}</span>
          <NSelect
            :value="
              activeDraft.failover?.enabled === undefined
                ? ''
                : String(activeDraft.failover.enabled)
            "
            :options="boolOptions"
            @update:value="
              (val) =>
                setBoolField(
                  ensureFailover(activeDraft) as Record<string, unknown>,
                  'enabled',
                  val,
                )
            "
          />
        </div>

        <div class="phase-d__field">
          <span class="phase-d__label">{{ t('failoverAfter') }}</span>
          <NInput
            :value="activeDraft.failover?.after ?? ''"
            placeholder="60s"
            clearable
            @update:value="
              (val) =>
                setStrField(
                  ensureFailover(activeDraft) as Record<string, unknown>,
                  'after',
                  val,
                )
            "
          />
        </div>

        <div class="phase-d__field">
          <span class="phase-d__label">{{
            t('requireSignedContentType')
          }}</span>
          <NSelect
            :value="
              activeDraft.requireSignedContentType === undefined
                ? ''
                : String(activeDraft.requireSignedContentType)
            "
            :options="boolOptions"
            @update:value="
              (val) =>
                setBoolField(
                  activeDraft as Record<string, unknown>,
                  'requireSignedContentType',
                  val,
                )
            "
          />
        </div>

        <div class="phase-d__field">
          <span class="phase-d__label">{{ t('readHeaderTimeout') }}</span>
          <NInput
            :value="activeDraft.readHeaderTimeout ?? ''"
            placeholder="10s"
            clearable
            @update:value="
              (val) =>
                setStrField(
                  activeDraft as Record<string, unknown>,
                  'readHeaderTimeout',
                  val,
                )
            "
          />
        </div>

        <div class="phase-d__field">
          <span class="phase-d__label">{{ t('writerLease') }}</span>
          <NInput
            :value="activeDraft.writerLease ?? ''"
            placeholder="30s"
            clearable
            @update:value="
              (val) =>
                setStrField(
                  activeDraft as Record<string, unknown>,
                  'writerLease',
                  val,
                )
            "
          />
        </div>
      </div>

      <Callout
        v-if="fleetConflict"
        tone="danger"
      >
        {{ t('fleetConflict') }}
        <NButton
          size="tiny"
          @click="reloadFleetConfig"
          >{{ t('fleetReload') }}</NButton
        >
      </Callout>

      <div class="phase-d__actions">
        <NButton
          type="primary"
          :loading="savingFleet"
          @click="saveFleetConfig"
          >{{ t('save') }}</NButton
        >
        <StatusTag
          v-if="savedFleet"
          variant="success"
          >{{ t('saved') }}</StatusTag
        >
      </div>
    </SectionCard>
    <Callout>{{ t('settingsHint') }}</Callout>
  </div>
</template>

<style lang="scss" src="@/components/tutti/users/phaseD.scss"></style>
