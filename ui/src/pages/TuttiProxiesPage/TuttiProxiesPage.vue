<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  import { computed, onMounted, onBeforeUnmount, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NButton } from 'naive-ui';
  import PageHeader from '@/components/layout/PageHeader.vue';
  import SectionCard from '@/components/ui/SectionCard.vue';
  import StatusTag from '@/components/ui/StatusTag.vue';
  import Callout from '@/components/ui/Callout.vue';
  import EmptyState from '@/components/ui/EmptyState.vue';
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

    if (!groupsDraft.value[editingScope.value]) {
      groupsDraft.value[editingScope.value] = {};
    }

    return groupsDraft.value[editingScope.value]!;
  });

  function cloneOverride(
    src?: TuttiProxySettingsOverride,
  ): TuttiProxySettingsOverride {
    return src ? (JSON.parse(JSON.stringify(src)) as TuttiProxySettingsOverride) : {};
  }

  function hydrateFleetDraft(fc: TuttiProxyFleetConfig) {
    fleetConfig.value = fc;
    defaultsDraft.value = cloneOverride(fc.defaults);
    const nextGroups: Record<string, TuttiProxySettingsOverride> = {};

    Object.entries(fc.groups ?? {}).forEach(([group, ov]) => {
      nextGroups[group] = cloneOverride(ov);
    });
    groupsDraft.value = nextGroups;
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

  function setBoolField(
    target: Record<string, unknown>,
    key: string,
    val: string,
  ) {
    if (val === '') {
      delete target[key];
    } else {
      target[key] = val === 'true';
    }
  }

  function setStrField(
    target: Record<string, unknown>,
    key: string,
    val: string,
  ) {
    const trimmed = val.trim();

    if (!trimmed) {
      delete target[key];
    } else {
      target[key] = trimmed;
    }
  }

  function setNumField(
    target: Record<string, unknown>,
    key: string,
    val: string,
  ) {
    const trimmed = val.trim();

    if (!trimmed) {
      delete target[key];
    } else {
      const parsed = Number(trimmed);

      if (Number.isFinite(parsed)) {
        target[key] = parsed;
      }
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
      });

      hydrateFleetDraft(updated);
      configVersion.value = updated.version ?? configVersion.value;
      savedFleet.value = true;
      await load();
    } catch {
      error.value = true;
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
      <div class="phase-d__toolbar">
        <NButton
          :type="editingScope === '__defaults__' ? 'primary' : 'default'"
          size="small"
          @click="editingScope = '__defaults__'"
          >{{ t('clusterDefaults') }}</NButton
        >
        <NButton
          v-for="grp in groupNames"
          :key="grp"
          :type="editingScope === grp ? 'primary' : 'default'"
          size="small"
          @click="editingScope = grp"
          >{{ grp }}</NButton
        >
        <input
          v-model="newGroupName"
          class="phase-d__input"
          :placeholder="t('groupName')"
          @keyup.enter="addGroupOverride"
        />
        <NButton
          size="small"
          @click="addGroupOverride"
          >{{ t('addGroup') }}</NButton
        >
        <NButton
          v-if="editingScope !== '__defaults__'"
          size="small"
          @click="removeGroupOverride(editingScope)"
          >{{ t('remove') }} ({{ editingScope }})</NButton
        >
      </div>

      <div class="phase-d__grid">
        <label class="phase-d__field">
          <span>{{ t('sync') }}</span>
          <select
            :value="
              activeDraft.replication?.sync?.enabled === undefined
                ? ''
                : String(activeDraft.replication.sync.enabled)
            "
            @change="
              setBoolField(
                ensureSync(activeDraft) as Record<string, unknown>,
                'enabled',
                ($event.target as HTMLSelectElement).value,
              )
            "
          >
            <option value="">{{ t('inherit') }}</option>
            <option value="true">{{ t('on') }}</option>
            <option value="false">{{ t('off') }}</option>
          </select>
        </label>

        <label class="phase-d__field">
          <span>{{ t('writeQuorum') }}</span>
          <input
            type="number"
            min="0"
            :placeholder="t('inherit')"
            :value="activeDraft.replication?.sync?.writeQuorum ?? ''"
            @input="
              setNumField(
                ensureSync(activeDraft) as Record<string, unknown>,
                'writeQuorum',
                ($event.target as HTMLInputElement).value,
              )
            "
          />
        </label>

        <label class="phase-d__field">
          <span>{{ t('syncTimeout') }}</span>
          <input
            type="text"
            placeholder="30s"
            :value="activeDraft.replication?.sync?.timeout ?? ''"
            @input="
              setStrField(
                ensureSync(activeDraft) as Record<string, unknown>,
                'timeout',
                ($event.target as HTMLInputElement).value,
              )
            "
          />
        </label>

        <label class="phase-d__field">
          <span>{{ t('onQuorumFailure') }}</span>
          <select
            :value="activeDraft.replication?.sync?.onQuorumFailure ?? ''"
            @change="
              setStrField(
                ensureSync(activeDraft) as Record<string, unknown>,
                'onQuorumFailure',
                ($event.target as HTMLSelectElement).value,
              )
            "
          >
            <option value="">{{ t('inherit') }}</option>
            <option value="fail">fail</option>
            <option value="accept">accept</option>
          </select>
        </label>

        <label class="phase-d__field">
          <span>{{ t('onEventError') }}</span>
          <select
            :value="activeDraft.replication?.onEventError ?? ''"
            @change="
              setStrField(
                ensureReplication(activeDraft) as Record<string, unknown>,
                'onEventError',
                ($event.target as HTMLSelectElement).value,
              )
            "
          >
            <option value="">{{ t('inherit') }}</option>
            <option value="fail">fail</option>
            <option value="log">log</option>
          </select>
        </label>

        <label class="phase-d__field">
          <span>{{ t('failureThreshold') }}</span>
          <input
            type="number"
            min="1"
            :placeholder="t('inherit')"
            :value="activeDraft.health?.failureThreshold ?? ''"
            @input="
              setNumField(
                ensureHealth(activeDraft) as Record<string, unknown>,
                'failureThreshold',
                ($event.target as HTMLInputElement).value,
              )
            "
          />
        </label>

        <label class="phase-d__field">
          <span>{{ t('openDuration') }}</span>
          <input
            type="text"
            placeholder="30s"
            :value="activeDraft.health?.openDuration ?? ''"
            @input="
              setStrField(
                ensureHealth(activeDraft) as Record<string, unknown>,
                'openDuration',
                ($event.target as HTMLInputElement).value,
              )
            "
          />
        </label>

        <label class="phase-d__field">
          <span>{{ t('failover') }}</span>
          <select
            :value="
              activeDraft.failover?.enabled === undefined
                ? ''
                : String(activeDraft.failover.enabled)
            "
            @change="
              setBoolField(
                ensureFailover(activeDraft) as Record<string, unknown>,
                'enabled',
                ($event.target as HTMLSelectElement).value,
              )
            "
          >
            <option value="">{{ t('inherit') }}</option>
            <option value="true">{{ t('on') }}</option>
            <option value="false">{{ t('off') }}</option>
          </select>
        </label>

        <label class="phase-d__field">
          <span>{{ t('failoverAfter') }}</span>
          <input
            type="text"
            placeholder="60s"
            :value="activeDraft.failover?.after ?? ''"
            @input="
              setStrField(
                ensureFailover(activeDraft) as Record<string, unknown>,
                'after',
                ($event.target as HTMLInputElement).value,
              )
            "
          />
        </label>

        <label class="phase-d__field">
          <span>{{ t('requireSignedContentType') }}</span>
          <select
            :value="
              activeDraft.requireSignedContentType === undefined
                ? ''
                : String(activeDraft.requireSignedContentType)
            "
            @change="
              setBoolField(
                activeDraft as Record<string, unknown>,
                'requireSignedContentType',
                ($event.target as HTMLSelectElement).value,
              )
            "
          >
            <option value="">{{ t('inherit') }}</option>
            <option value="true">{{ t('on') }}</option>
            <option value="false">{{ t('off') }}</option>
          </select>
        </label>

        <label class="phase-d__field">
          <span>{{ t('readHeaderTimeout') }}</span>
          <input
            type="text"
            placeholder="10s"
            :value="activeDraft.readHeaderTimeout ?? ''"
            @input="
              setStrField(
                activeDraft as Record<string, unknown>,
                'readHeaderTimeout',
                ($event.target as HTMLInputElement).value,
              )
            "
          />
        </label>

        <label class="phase-d__field">
          <span>{{ t('writerLease') }}</span>
          <input
            type="text"
            placeholder="30s"
            :value="activeDraft.writerLease ?? ''"
            @input="
              setStrField(
                activeDraft as Record<string, unknown>,
                'writerLease',
                ($event.target as HTMLInputElement).value,
              )
            "
          />
        </label>
      </div>

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
