<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  import { computed, onMounted, onBeforeUnmount, ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { useRouter } from 'vue-router';
  import { NButton, NInput, NModal, NCheckbox } from 'naive-ui';
  import PageHeader from '@/components/layout/PageHeader.vue';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import SectionCard from '@/components/ui/SectionCard.vue';
  import StatusTag from '@/components/ui/StatusTag.vue';
  import Callout from '@/components/ui/Callout.vue';
  import BackLink from '@/components/tutti/common/BackLink.vue';
  import TabBar from '@/components/tutti/common/TabBar.vue';
  import ProxyAccountKeysTable from '@/components/tutti/accounts/ProxyAccountKeysTable.vue';
  import ProxyKeyRotateModal from '@/components/tutti/accounts/ProxyKeyRotateModal.vue';
  import ProxySecretModal from '@/components/tutti/accounts/ProxySecretModal.vue';
  import AccessPolicyDrawer from '@/components/tutti/accounts/AccessPolicyDrawer.vue';
  import messages from '@/components/tutti/users/i18nManagement';
  import { TuttiService } from '@/services/TuttiService';
  import { useTuttiUsersStore } from '@/stores/tuttiUsersStore';
  import { useTuttiAccessPolicyStore } from '@/stores/tuttiAccessPolicyStore';
  import { useTuttiProtectionStore } from '@/stores/tuttiProtectionStore';
  import { useQueryTab } from '@/utils/composables/useQueryTab';
  import { UserHelper } from '@/utils/helpers/UserHelper';
  import { ProxyAccountHelper } from '@/utils/helpers/ProxyAccountHelper';
  import { ProtectionGroupHelper } from '@/utils/helpers/ProtectionGroupHelper';
  import type { TuttiProxyAccountSecret } from '@/utils/types/tutti';
  import { RouteName } from '@/utils/types/router';
  const props = defineProps<{ name: string }>();
  const { t } = useI18n({ messages });
  const store = useTuttiUsersStore();
  const keys = useTuttiAccessPolicyStore();
  const protectionStore = useTuttiProtectionStore();
  const router = useRouter();
  const tab = useQueryTab(
    ['credentials', 'keys', 'permissions'] as const,
    'credentials',
  );
  const user = computed(() => store.find(props.name));
  const account = computed(
    () => keys.accounts.find((a) => a.user === props.name) ?? null,
  );
  const isConfig = computed(
    () => user.value?.storages?.some((s) => s.source === 'config') ?? false,
  );

  const userBuckets = computed(() => user.value?.policy?.buckets ?? []);
  const isWildcardBuckets = computed(
    () => userBuckets.value.length === 0 || userBuckets.value.includes('*'),
  );

  const relatedProtectionGroups = computed(() => {
    const allGroups = protectionStore.groups;
    const uName = props.name;
    const bList = userBuckets.value;

    return allGroups.filter((g) => {
      if (g.user === uName) return true;

      if (isWildcardBuckets.value) return true;

      if ((g.buckets ?? []).length > 0) {
        return (g.buckets ?? []).some((b) => bList.includes(b));
      }

      return false;
    });
  });

  const accessibleStorages = computed(() => {
    if (!user.value?.storages) return [];

    const allowedStorageNames = new Set<string>();

    for (const group of relatedProtectionGroups.value) {
      if (group.primary) allowedStorageNames.add(group.primary);

      for (const m of group.members ?? []) {
        if (m) allowedStorageNames.add(m);
      }
    }

    return user.value.storages.filter((entry) => {
      const mode = UserHelper.modeOf(entry);

      if (mode === 'override') return true;

      if (mode === 'missing') return false;

      if (isWildcardBuckets.value) return mode === 'connection-account';

      return (
        allowedStorageNames.has(entry.storage) &&
        mode === 'connection-account'
      );
    });
  });

  const error = ref('');
  const busy = ref(false);
  const showDelete = ref(false);
  const force = ref(false);
  const canForce = ref(false);
  const editStorage = ref<string | null>(null);
  const accessKey = ref('');
  const secretKey = ref('');
  const showPolicy = ref(false);
  const secret = ref<TuttiProxyAccountSecret | null>(null);
  const secretKind = ref<'created' | 'rotated'>('created');
  const rotateId = ref<string | null>(null);
  const testResults = ref<Record<string, string>>({});
  const tabs = computed(() => [
    { value: 'credentials', label: t('credentials') },
    {
      value: 'keys',
      label: t('keys'),
      count: account.value
        ? ProxyAccountHelper.getKeyRows(account.value).length
        : 0,
    },
    { value: 'permissions', label: t('permission') },
  ]);

  async function load() {
    await Promise.all([store.load(), keys.init(), protectionStore.refresh()]);
  }
  onMounted(load);
  watch(
    () => props.name,
    () => {
      error.value = '';
      testResults.value = {};
      load();
    },
  );
  onBeforeUnmount(() => {
    secret.value = null;
    secretKey.value = '';
  });

  function edit(storage: string) {
    editStorage.value = storage;
    accessKey.value = '';
    secretKey.value = '';
    error.value = '';
  }

  async function saveCredential() {
    if (!editStorage.value || !accessKey.value || !secretKey.value) return;

    busy.value = true;
    error.value = '';

    try {
      await TuttiService.updateUserCredentials({
        name: props.name,
        storage: editStorage.value,
        accessKeyId: accessKey.value,
        secretAccessKey: secretKey.value,
        verify: true,
      });
      editStorage.value = null;
      secretKey.value = '';
      await load();
    } catch (e) {
      error.value = ProxyAccountHelper.getMessage(e) ?? t('error');
    } finally {
      busy.value = false;
    }
  }

  const removeStorage = ref<string | null>(null);

  async function removeOverride() {
    if (!removeStorage.value) return;

    busy.value = true;
    error.value = '';

    try {
      await TuttiService.removeUserCredential(props.name, removeStorage.value);
      removeStorage.value = null;
      await load();
    } catch (e) {
      error.value = ProxyAccountHelper.getMessage(e) ?? t('error');
    } finally {
      busy.value = false;
    }
  }

  async function test(storage: string) {
    busy.value = true;

    try {
      const result = await TuttiService.testUser({ name: props.name, storage });

      testResults.value[storage] = result.ok
        ? t('verified', { latency: result.latencyMs ?? 0 })
        : (result.error ?? t('failed'));
    } catch (e) {
      testResults.value[storage] =
        ProxyAccountHelper.getMessage(e) ?? t('failed');
    } finally {
      busy.value = false;
    }
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
</script>

<template>
  <div class="phase-d">
    <BackLink
      :to="{ name: RouteName.TUTTI_USERS }"
      :label="t('users')"
    /><PageHeader
      :title="name"
      :description="t('usersHint')"
      ><template #actions
        ><WriteGate
          ><NButton
            type="error"
            secondary
            :disabled="isConfig || !user"
            @click="
              showDelete = true;
              error = '';
              force = false;
              canForce = false;
            "
            >{{ t('delete') }}</NButton
          ><NButton
            type="primary"
            :loading="busy"
            :disabled="!user || !store.dynamicEnabled"
            @click="createKey"
            >{{
              user && user.accessKeys?.dynamicActive
                ? t('newKey')
                : t('firstKey')
            }}</NButton
          ></WriteGate
        ></template
      ></PageHeader
    >
    <Callout
      v-if="error && !showDelete && !editStorage && !removeStorage"
      tone="danger"
      >{{ error }}</Callout
    ><Callout
      v-if="store.hasError || keys.hasError"
      tone="danger"
      >{{ t('error') }}
      <NButton
        text
        @click="load"
        >{{ t('retry') }}</NButton
      ></Callout
    ><Callout
      v-if="store.isLoaded && !user"
      tone="warning"
      >{{ t('userNotFound') }}</Callout
    >
    <template v-if="user"
      ><div class="phase-d__actions">
        <StatusTag variant="success">{{ t('active') }}</StatusTag>
        <StatusTag>{{ t(isConfig ? 'config' : 'managed') }}</StatusTag>
      </div>
      <TabBar
        v-model="tab"
        :label="name"
        :tabs="tabs"
      />
      <SectionCard
        v-if="tab === 'credentials'"
        :title="t('credentials')"
        flush
        ><p
          style="padding: 16px"
          class="phase-d__muted"
        >
          {{ t('credentialHint') }}
        </p>
        <table
          v-if="accessibleStorages.length"
          class="phase-d__table"
        >
          <thead>
            <tr>
              <th>{{ t('storages') }}</th>
              <th>{{ t('mode') }}</th>
              <th>{{ t('accessKeyId') }}</th>
              <th>{{ t('source') }}</th>
              <th>{{ t('actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="entry in accessibleStorages"
              :key="entry.storage"
            >
              <td>
                <RouterLink
                  :to="{
                    name: RouteName.CHORUS_STORAGE_DETAILS,
                    params: { storageName: entry.storage },
                  }"
                  >{{ entry.storage }}</RouterLink
                >
              </td>
              <td>
                <StatusTag
                  :variant="
                    UserHelper.modeOf(entry) === 'connection-account'
                      ? 'success'
                      : UserHelper.modeOf(entry) === 'override'
                        ? 'info'
                        : 'warning'
                  "
                  >{{
                    t(
                      {
                        'connection-account': 'modeConnectionLabel',
                        override: 'modeOverrideLabel',
                        missing: 'modeMissingLabel',
                      }[UserHelper.modeOf(entry)],
                    )
                  }}</StatusTag
                >
                <div class="phase-d__muted">
                  {{ testResults[entry.storage] }}
                </div>
              </td>
              <td>
                <code>{{ entry.accessKeyId || '–' }}</code>
              </td>
              <td>{{ t(entry.source === 'config' ? 'config' : 'managed') }}</td>
              <td>
                <div class="phase-d__actions">
                  <WriteGate
                    ><NButton
                      size="small"
                      :disabled="busy || UserHelper.modeOf(entry) === 'missing'"
                      @click="test(entry.storage)"
                      >{{ t('test') }}</NButton
                    ><NButton
                      size="small"
                      :disabled="
                        entry.source === 'config' || !store.dynamicEnabled
                      "
                      @click="edit(entry.storage)"
                      >{{
                        t(
                          UserHelper.modeOf(entry) === 'override'
                            ? 'update'
                            : 'add',
                        )
                      }}</NButton
                    ><NButton
                      v-if="
                        UserHelper.modeOf(entry) === 'override' &&
                        entry.source !== 'config'
                      "
                      size="small"
                      secondary
                      type="error"
                      :disabled="busy || !store.dynamicEnabled"
                      @click="removeStorage = entry.storage"
                      >{{ t('removeOverride') }}</NButton
                    ></WriteGate
                  >
                </div>
              </td>
            </tr>
          </tbody>
        </table>
        <div
          v-else
          style="padding: 24px; text-align: center"
          class="phase-d__muted"
        >
          No storage credentials or protection group access assigned to this user.
        </div></SectionCard
      >
      <SectionCard
        v-else-if="tab === 'keys'"
        :title="t('keys')"
        ><ProxyAccountKeysTable
          v-if="account"
          :account="account"
          @rotate="rotateId = $event"
        />
        <p v-else>{{ t('keyHint') }}</p></SectionCard
      >
      <div
        v-else-if="tab === 'permissions'"
        style="display: flex; flex-direction: column; gap: 16px"
      >
        <SectionCard :title="t('permission')">
          <div class="phase-d__stack">
            <div>
              <p
                class="phase-d__muted"
                style="margin-bottom: 8px"
              >
                S3 Access Policy specifies the operations (Read/Write) this user's credentials can perform on specific buckets.
              </p>
              <div style="display: flex; align-items: center; gap: 12px">
                <StatusTag :variant="user.policy?.permission === 'read-only' ? 'info' : 'success'">
                  {{ user.policy?.permission || 'read-write' }}
                </StatusTag>
                <span>Buckets: <code>{{ user.policy?.buckets?.join(', ') || '*' }}</code></span>
                <span
                  v-if="isWildcardBuckets"
                  class="phase-d__muted"
                >(Access to all current and future buckets)</span>
              </div>
            </div>
            <WriteGate>
              <NButton @click="showPolicy = true">{{
                t('editPolicy')
              }}</NButton>
            </WriteGate>
          </div>
        </SectionCard>

        <SectionCard
          title="Protection Groups Coverage"
          flush
        >
          <div style="padding: 16px 16px 8px">
            <p
              class="phase-d__muted"
              style="margin: 0"
            >
              When this user writes or reads objects through Tutti Proxy, requests are protected, fenced, and replicated according to the Protection Groups covering those buckets.
            </p>
          </div>
          <table
            v-if="relatedProtectionGroups.length"
            class="phase-d__table"
          >
            <thead>
              <tr>
                <th>Protection Group</th>
                <th>Scope Kind</th>
                <th>Primary Storage</th>
                <th>Covered Buckets</th>
                <th>Health Status</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="group in relatedProtectionGroups"
                :key="group.id"
              >
                <td>
                  <RouterLink
                    :to="{
                      name: RouteName.TUTTI_PROTECTION_GROUP_DETAIL,
                      params: { id: group.id },
                    }"
                  >
                    <strong>{{ group.id }}</strong>
                  </RouterLink>
                </td>
                <td>
                  <StatusTag variant="info">
                    {{
                      ProtectionGroupHelper.scopeKind(group) === 'storage'
                        ? 'Storage-level'
                        : ProtectionGroupHelper.scopeKind(group) === 'buckets'
                          ? 'Bucket-level'
                          : 'User-level'
                    }}
                  </StatusTag>
                </td>
                <td>
                  <strong>{{ group.primary }}</strong>
                </td>
                <td>
                  <code>{{ (group.buckets && group.buckets.length > 0) ? group.buckets.join(', ') : '–' }}</code>
                </td>
                <td>
                  <StatusTag
                    :variant="
                      ProtectionGroupHelper.stateVariant(
                        ProtectionGroupHelper.displayState(group),
                      )
                    "
                  >
                    {{ ProtectionGroupHelper.displayState(group) }}
                  </StatusTag>
                </td>
                <td>
                  <RouterLink
                    :to="{
                      name: RouteName.TUTTI_PROTECTION_GROUP_DETAIL,
                      params: { id: group.id },
                    }"
                  >
                    View Group
                  </RouterLink>
                </td>
              </tr>
            </tbody>
          </table>
          <div
            v-else
            style="padding: 24px; text-align: center"
            class="phase-d__muted"
          >
            No Protection Groups currently assigned to this user.
          </div>
        </SectionCard>
      </div>
    </template>
    <NModal
      :show="editStorage !== null"
      preset="card"
      :title="t('add')"
      style="width: 560px; max-width: 94vw"
      @update:show="!$event && ((editStorage = null), (secretKey = ''))"
      ><div class="phase-d phase-d__form">
        <Callout
          v-if="error"
          tone="danger"
          >{{ error }}</Callout
        >
        <form
          class="phase-d__form"
          @submit.prevent="saveCredential"
        >
          <label class="phase-d__field"
            >{{ t('accessKeyId')
            }}<NInput
              v-model:value="accessKey"
              :input-props="{
                'aria-label': t('accessKeyId'),
                autocomplete: 'off',
              }" /></label
          ><label class="phase-d__field"
            >{{ t('secret')
            }}<NInput
              v-model:value="secretKey"
              type="password"
              :input-props="{
                'aria-label': t('secret'),
                autocomplete: 'new-password',
              }" /></label
          ><NButton
            type="primary"
            attr-type="submit"
            :loading="busy"
            :disabled="!accessKey || !secretKey"
            >{{ t('verify') }} &amp; {{ t('save') }}</NButton
          >
        </form>
      </div></NModal
    >
    <NModal
      :show="removeStorage !== null"
      preset="card"
      :title="t('removeOverride')"
      style="width: 520px; max-width: 94vw"
      @update:show="!$event && (removeStorage = null)"
      ><div class="phase-d">
        <p>
          {{
            t('removeOverrideConfirm', {
              name,
              storage: removeStorage ?? '',
            })
          }}
        </p>
        <Callout
          v-if="error"
          tone="danger"
          >{{ error }}</Callout
        >
        <div class="phase-d__actions">
          <NButton @click="removeStorage = null">{{ t('cancel') }}</NButton
          ><NButton
            type="error"
            :loading="busy"
            @click="removeOverride"
            >{{ t('removeOverride') }}</NButton
          >
        </div>
      </div></NModal
    >
    <NModal
      v-model:show="showDelete"
      preset="card"
      :title="t('delete')"
      style="width: 520px; max-width: 94vw"
      ><div class="phase-d">
        <p>{{ t('confirmDelete', { name }) }}</p>
        <Callout
          v-if="error"
          tone="danger"
          >{{ error }}</Callout
        ><NCheckbox
          v-if="canForce"
          v-model:checked="force"
          >{{ t('force') }}</NCheckbox
        >
        <div class="phase-d__actions">
          <NButton @click="showDelete = false">{{ t('cancel') }}</NButton
          ><NButton
            type="error"
            :loading="busy"
            @click="remove"
            >{{ t('delete') }}</NButton
          >
        </div>
      </div></NModal
    >
    <AccessPolicyDrawer
      v-model:show="showPolicy"
      :account="account ?? { user: name }"
      @update:show="!$event && load()"
    /><ProxyKeyRotateModal
      :access-key-id="rotateId"
      @close="rotateId = null"
      @rotated="
        secretKind = 'rotated';
        secret = $event;
        rotateId = null;
      "
    /><ProxySecretModal
      :secret="secret"
      :kind="secretKind"
      @close="
        secret = null;
        load();
      "
    />
  </div>
</template>

<style lang="scss" src="@/components/tutti/users/phaseD.scss"></style>
