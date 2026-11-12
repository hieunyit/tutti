<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  import { computed, onMounted, onBeforeUnmount, ref } from 'vue';
  import { useRouter } from 'vue-router';
  import { useI18n } from 'vue-i18n';
  import {
    NAlert,
    NButton,
    NCheckbox,
    NInput,
    NRadio,
    NSelect,
    NSteps,
    NStep,
    NTag,
  } from 'naive-ui';
  import PageHeader from '@/components/layout/PageHeader.vue';
  import BackLink from '@/components/tutti/common/BackLink.vue';
  import SectionCard from '@/components/ui/SectionCard.vue';
  import Callout from '@/components/ui/Callout.vue';
  import ProxySecretModal from '@/components/tutti/accounts/ProxySecretModal.vue';
  import messages from '@/components/tutti/users/i18nManagement';
  import { TuttiService } from '@/services/TuttiService';
  import { useTuttiUsersStore } from '@/stores/tuttiUsersStore';
  import { useTuttiProtectionStore } from '@/stores/tuttiProtectionStore';
  import { UserHelper } from '@/utils/helpers/UserHelper';
  import { ProxyAccountHelper } from '@/utils/helpers/ProxyAccountHelper';
  import { ProtectionGroupHelper } from '@/utils/helpers/ProtectionGroupHelper';
  import { AccessPolicyHelper } from '@/utils/helpers/AccessPolicyHelper';
  import {
    BucketDiscoveryHelper,
    type StorageBucketEntry,
  } from '@/utils/helpers/BucketDiscoveryHelper';
  import type { TuttiProxyAccountSecret } from '@/utils/types/tutti';
  import { RouteName } from '@/utils/types/router';

  interface GroupBucketItem {
    bucket: string;
    groupId: string;
  }

  const { t } = useI18n({ messages });
  const store = useTuttiUsersStore();
  const protectionStore = useTuttiProtectionStore();
  const router = useRouter();

  const step = ref(1);
  const name = ref('');
  const busy = ref(false);
  const error = ref('');
  const createKey = ref(true);
  const permission = ref('read-write');
  const grantMode = ref<'all' | 'groups' | 'custom'>('all');
  const selectedGroupIds = ref<string[]>([]);
  const selectedGroupBuckets = ref<string[]>([]);
  const customExtraBuckets = ref<string[]>([]);
  const clusterBuckets = ref<string[]>([]);
  const discoveredStorageBuckets = ref<StorageBucketEntry[]>([]);
  const extraPatternInput = ref('');
  const buckets = ref('*');
  const description = ref('');
  const secret = ref<TuttiProxyAccountSecret | null>(null);
  const verticalSteps = ref(false);
  let mobileQuery: MediaQueryList | undefined;

  const permissionOptions = computed(() => [
    { label: t('readWrite'), value: 'read-write' },
    { label: t('readOnly'), value: 'read-only' },
    { label: t('writeOnly'), value: 'write-only' },
  ]);

  const protectionGroupOptions = computed(() =>
    protectionStore.groups.map((group) => {
      const scope = ProtectionGroupHelper.scopeKind(group);
      const details =
        scope === 'storage'
          ? `All buckets on ${group.primary || (group.members ?? []).join(', ')}`
          : (group.buckets ?? []).join(', ') || 'No buckets';

      return {
        label: `${group.id} (${details})`,
        value: group.id,
      };
    }),
  );

  const availableGroupBuckets = computed<GroupBucketItem[]>(() => {
    const selectedGroups = protectionStore.groups.filter((g) =>
      selectedGroupIds.value.includes(g.id),
    );
    const items: GroupBucketItem[] = [];
    const seen = new Set<string>();

    for (const g of selectedGroups) {
      for (const b of g.buckets ?? []) {
        if (!seen.has(b)) {
          seen.add(b);
          items.push({ bucket: b, groupId: g.id });
        }
      }
    }

    for (const entry of discoveredStorageBuckets.value) {
      if (!seen.has(entry.bucket)) {
        seen.add(entry.bucket);
        items.push({
          bucket: entry.bucket,
          groupId: `storage: ${entry.storage}`,
        });
      }
    }

    return items;
  });

  const storageScopeGroups = computed(() =>
    protectionStore.groups.filter(
      (g) =>
        selectedGroupIds.value.includes(g.id) &&
        ProtectionGroupHelper.scopeKind(g) === 'storage',
    ),
  );

  async function loadClusterBuckets() {
    try {
      clusterBuckets.value = await BucketDiscoveryHelper.discoverBuckets();
    } catch {
      clusterBuckets.value = [];
    }
  }

  async function discoverBucketsForStorageGroups() {
    const storagesToQuery = new Set<string>();

    for (const g of storageScopeGroups.value) {
      if (g.primary) storagesToQuery.add(g.primary);

      for (const m of g.members ?? []) {
        storagesToQuery.add(m);
      }
    }

    if (storagesToQuery.size === 0) {
      discoveredStorageBuckets.value = [];

      return;
    }

    try {
      discoveredStorageBuckets.value =
        await BucketDiscoveryHelper.discoverStorageBuckets(
          Array.from(storagesToQuery),
        );
    } catch {
      discoveredStorageBuckets.value = [];
    }
  }

  function addSuggestedBucket(b: string) {
    if (
      !buckets.value ||
      buckets.value.trim() === '*' ||
      buckets.value.trim() === ''
    ) {
      buckets.value = b;

      return;
    }

    const current = buckets.value
      .split(',')
      .map((x) => x.trim())
      .filter(Boolean);

    if (!current.includes(b)) {
      current.push(b);
      buckets.value = current.join(', ');
    }
  }

  async function onGroupsChange(ids: string[]) {
    const prevGroups = selectedGroupIds.value;

    selectedGroupIds.value = ids;
    await discoverBucketsForStorageGroups();

    const currentAvailable = new Set(
      availableGroupBuckets.value.map((b) => b.bucket),
    );
    const kept = selectedGroupBuckets.value.filter((b) =>
      currentAvailable.has(b),
    );

    const addedGroupIds = ids.filter((id) => !prevGroups.includes(id));
    const newBucketsToAdd: string[] = [];

    for (const gid of addedGroupIds) {
      const grp = protectionStore.groups.find((g) => g.id === gid);

      for (const b of grp?.buckets ?? []) {
        if (!kept.includes(b) && !newBucketsToAdd.includes(b)) {
          newBucketsToAdd.push(b);
        }
      }
    }

    for (const entry of discoveredStorageBuckets.value) {
      if (!kept.includes(entry.bucket) && !newBucketsToAdd.includes(entry.bucket)) {
        newBucketsToAdd.push(entry.bucket);
      }
    }

    selectedGroupBuckets.value = [...kept, ...newBucketsToAdd];
  }

  function toggleGroupBucket(bucket: string, checked: boolean) {
    if (checked) {
      if (!selectedGroupBuckets.value.includes(bucket)) {
        selectedGroupBuckets.value.push(bucket);
      }
    } else {
      selectedGroupBuckets.value = selectedGroupBuckets.value.filter(
        (b) => b !== bucket,
      );
    }
  }

  function selectAllGroupBuckets() {
    selectedGroupBuckets.value = availableGroupBuckets.value.map(
      (b) => b.bucket,
    );
  }

  function deselectAllGroupBuckets() {
    selectedGroupBuckets.value = [];
  }

  function addExtraPattern() {
    const pat = extraPatternInput.value.trim();

    if (!pat) return;

    const err = AccessPolicyHelper.validatePattern(pat);

    if (err) {
      error.value = t('invalidBuckets');

      return;
    }

    if (!customExtraBuckets.value.includes(pat)) {
      customExtraBuckets.value.push(pat);
    }

    extraPatternInput.value = '';
    error.value = '';
  }

  function removeExtraPattern(pattern: string) {
    customExtraBuckets.value = customExtraBuckets.value.filter(
      (p) => p !== pattern,
    );
  }

  const effectiveBucketsDisplay = computed(() => {
    if (grantMode.value === 'all') return '* (All buckets)';

    if (grantMode.value === 'groups') {
      if (
        storageScopeGroups.value.length > 0 &&
        selectedGroupBuckets.value.length === 0 &&
        customExtraBuckets.value.length === 0
      ) {
        return '* (All buckets via storage-scoped group)';
      }

      const combined = [
        ...selectedGroupBuckets.value,
        ...customExtraBuckets.value,
      ];

      return combined.join(', ') || t('noBucketsInSelectedGroups');
    }

    return buckets.value.trim() || '*';
  });

  onMounted(async () => {
    mobileQuery = window.matchMedia('(max-width: 600px)');
    verticalSteps.value = mobileQuery.matches;
    mobileQuery.addEventListener('change', updateMobileSteps);
    loadClusterBuckets();
    await Promise.all([
      store.load(),
      protectionStore.refresh().catch(() => null),
    ]);
  });

  onBeforeUnmount(() => {
    mobileQuery?.removeEventListener('change', updateMobileSteps);
    secret.value = null;
  });

  function updateMobileSteps() {
    if (mobileQuery) verticalSteps.value = mobileQuery.matches;
  }

  function advance() {
    error.value = '';

    if (
      step.value === 1 &&
      UserHelper.validateName(
        name.value,
        store.users.map((u) => u.name),
      )
    ) {
      error.value = t('invalidName');

      return;
    }

    if (step.value === 2) {
      if (grantMode.value === 'groups') {
        if (selectedGroupIds.value.length === 0) {
          error.value = t('noGroupSelectedHint');

          return;
        }

        if (
          storageScopeGroups.value.length === 0 &&
          selectedGroupBuckets.value.length === 0 &&
          customExtraBuckets.value.length === 0
        ) {
          error.value = t('bucketsRequired');

          return;
        }
      } else if (grantMode.value === 'custom') {
        const problem = UserHelper.validatePolicy(buckets.value, true);

        if (problem) {
          error.value = t(problem);

          return;
        }
      }
    }

    step.value++;
  }

  async function submit() {
    let finalBuckets: string[] = ['*'];

    if (grantMode.value === 'all') {
      finalBuckets = ['*'];
    } else if (grantMode.value === 'groups') {
      if (
        storageScopeGroups.value.length > 0 &&
        selectedGroupBuckets.value.length === 0 &&
        customExtraBuckets.value.length === 0
      ) {
        finalBuckets = ['*'];
      } else {
        const combined = [
          ...selectedGroupBuckets.value,
          ...customExtraBuckets.value,
        ];

        finalBuckets = combined.length > 0 ? combined : ['*'];
      }
    } else {
      const parsed = UserHelper.parseBucketPatterns(buckets.value);

      if (parsed.invalid.length > 0) {
        error.value = t('invalidBuckets');

        return;
      }

      finalBuckets = parsed.patterns.length > 0 ? parsed.patterns : ['*'];
    }

    busy.value = true;
    error.value = '';

    try {
      const result = await TuttiService.createUser({
        name: name.value,
        credentials: [],
        verify: false,
        createAccessKey: createKey.value,
        description: description.value,
        policy: {
          user: name.value,
          permission: permission.value,
          buckets: finalBuckets,
        },
      });

      if (result.accessKey) secret.value = result.accessKey;
      else await finish();
    } catch (e) {
      error.value = ProxyAccountHelper.getMessage(e) ?? t('error');
    } finally {
      busy.value = false;
    }
  }

  async function finish() {
    secret.value = null;
    await router.push({
      name: RouteName.TUTTI_USER_DETAIL,
      params: { name: name.value },
    });
  }
</script>

<template>
  <div class="phase-d">
    <BackLink
      :to="{ name: RouteName.TUTTI_USERS }"
      :label="t('users')"
    />
    <PageHeader
      :title="t('createUser')"
      :description="t('createHint')"
    />
    <Callout
      v-if="!store.dynamicEnabled"
      tone="warning"
    >
      {{ t('notConfigured') }}
    </Callout>
    <Callout
      v-if="store.hasError && !error"
      tone="danger"
    >
      {{ t('error') }}
    </Callout>
    <NSteps
      :current="step"
      size="small"
      :vertical="verticalSteps"
    >
      <NStep :title="t('identity')" />
      <NStep :title="t('permission')" />
      <NStep :title="t('keys')" />
    </NSteps>
    <SectionCard
      :title="t(step === 1 ? 'identity' : step === 2 ? 'permission' : 'keys')"
    >
      <form
        class="phase-d__form"
        @submit.prevent="step === 3 ? submit() : advance()"
      >
        <template v-if="step === 1">
          <label class="phase-d__field">
            {{ t('name') }}
            <NInput
              v-model:value="name"
              :maxlength="64"
              :input-props="{ 'aria-label': t('name'), autocomplete: 'off' }"
            />
          </label>
          <p class="phase-d__muted">{{ t('nameHint') }}</p>
        </template>

        <template v-if="step === 2">
          <label class="phase-d__field">
            {{ t('permission') }}
            <NSelect
              v-model:value="permission"
              :options="permissionOptions"
            />
          </label>

          <div class="phase-d__field">
            <span>{{ t('grantModeLabel') }}</span>
            <div style="display: flex; flex-direction: column; gap: 8px; margin-top: 6px">
              <NRadio
                :checked="grantMode === 'all'"
                value="all"
                @update:checked="
                  grantMode = 'all';
                  buckets = '*';
                "
              >
                <strong>{{ t('grantModeAll') }}</strong>
                <p
                  class="phase-d__muted"
                  style="margin: 2px 0 0"
                >
                  {{ t('grantModeAllHint') }}
                </p>
              </NRadio>
              <NRadio
                :checked="grantMode === 'groups'"
                value="groups"
                @update:checked="grantMode = 'groups'"
              >
                <strong>{{ t('grantModeGroups') }}</strong>
                <p
                  class="phase-d__muted"
                  style="margin: 2px 0 0"
                >
                  {{ t('grantModeGroupsHint') }}
                </p>
              </NRadio>
              <NRadio
                :checked="grantMode === 'custom'"
                value="custom"
                @update:checked="
                  grantMode = 'custom';
                  if (buckets === '*') buckets = '';
                "
              >
                <strong>{{ t('grantModeCustom') }}</strong>
                <p
                  class="phase-d__muted"
                  style="margin: 2px 0 0"
                >
                  {{ t('grantModeCustomHint') }}
                </p>
              </NRadio>
            </div>
          </div>

          <template v-if="grantMode === 'groups'">
            <label class="phase-d__field">
              {{ t('selectProtectionGroups') }}
              <NSelect
                :value="selectedGroupIds"
                multiple
                filterable
                :options="protectionGroupOptions"
                :placeholder="t('selectProtectionGroupsPlaceholder')"
                @update:value="onGroupsChange"
              />
            </label>

            <NAlert
              v-for="grp in storageScopeGroups"
              :key="grp.id"
              type="info"
              style="margin-bottom: 12px"
            >
              {{
                t('storageScopeAlert', {
                  group: grp.id,
                  storage: grp.primary || (grp.members ?? []).join(', '),
                })
              }}
            </NAlert>

            <div
              v-if="selectedGroupIds.length > 0"
              class="phase-d__field"
            >
              <span>{{ t('groupBucketsLabel') }}</span>
              <p
                class="phase-d__muted"
                style="margin: 2px 0 8px"
              >
                {{ t('groupBucketsHint') }}
              </p>
              <div
                class="phase-d__panel"
                style="padding: 10px 12px"
              >
                <div
                  v-if="availableGroupBuckets.length > 0"
                  style="display: flex; gap: 8px; margin-bottom: 8px; padding-bottom: 6px; border-bottom: 1px dashed var(--tt-border, #e0e0e6)"
                >
                  <NButton
                    size="tiny"
                    quaternary
                    type="primary"
                    @click="selectAllGroupBuckets"
                  >
                    {{ t('selectAllBuckets') }}
                  </NButton>
                  <NButton
                    size="tiny"
                    quaternary
                    @click="deselectAllGroupBuckets"
                  >
                    {{ t('deselectAllBuckets') }}
                  </NButton>
                </div>

                <div
                  v-if="availableGroupBuckets.length > 0"
                  style="display: flex; flex-direction: column; gap: 8px; max-height: 200px; overflow-y: auto"
                >
                  <NCheckbox
                    v-for="item in availableGroupBuckets"
                    :key="item.bucket"
                    :checked="selectedGroupBuckets.includes(item.bucket)"
                    @update:checked="toggleGroupBucket(item.bucket, $event)"
                  >
                    <strong>{{ item.bucket }}</strong>
                    <span style="font-size: 11px; opacity: 0.65; margin-left: 4px">({{ item.groupId }})</span>
                  </NCheckbox>
                </div>
                <p
                  v-else
                  class="phase-d__muted"
                  style="margin: 0"
                >
                  {{ t('noBucketsInSelectedGroups') }}
                </p>
              </div>
            </div>

            <div
              v-if="selectedGroupIds.length > 0"
              class="phase-d__field"
            >
              <span>{{ t('additionalPatternsLabel') }}</span>
              <p
                class="phase-d__muted"
                style="margin: 2px 0 8px"
              >
                {{ t('additionalPatternsHint') }}
              </p>
              <div style="display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 8px">
                <NTag
                  v-for="pattern in customExtraBuckets"
                  :key="pattern"
                  closable
                  round
                  @close="removeExtraPattern(pattern)"
                >
                  {{ pattern }}
                </NTag>
              </div>
              <div style="display: flex; gap: 8px">
                <NInput
                  v-model:value="extraPatternInput"
                  :placeholder="t('bucketsHint')"
                  @keydown.enter.prevent="addExtraPattern"
                />
                <NButton
                  :disabled="!extraPatternInput.trim()"
                  @click="addExtraPattern"
                >
                  {{ t('add') }}
                </NButton>
              </div>
            </div>

            <p
              v-if="selectedGroupIds.length === 0"
              class="phase-d__muted"
            >
              {{ t('noGroupSelectedHint') }}
            </p>
          </template>

          <template v-if="grantMode === 'custom'">
            <div
              v-if="clusterBuckets.length > 0"
              class="phase-d__field"
            >
              <span>{{ t('clusterBucketsLabel') }}</span>
              <p
                class="phase-d__muted"
                style="margin: 2px 0 8px"
              >
                {{ t('clusterBucketsHint') }}
              </p>
              <div style="display: flex; flex-wrap: wrap; gap: 6px">
                <NTag
                  v-for="b in clusterBuckets"
                  :key="b"
                  round
                  checkable
                  :checked="buckets.split(',').map((x) => x.trim()).includes(b)"
                  @click="addSuggestedBucket(b)"
                >
                  + {{ b }}
                </NTag>
              </div>
            </div>

            <label class="phase-d__field">
              {{ t('buckets') }} *
              <NInput
                v-model:value="buckets"
                :input-props="{
                  'aria-label': t('buckets'),
                  'aria-required': true,
                }"
              />
            </label>
            <p class="phase-d__muted">
              {{ t('bucketsHint') }}
            </p>
          </template>
        </template>

        <template v-if="step === 3">
          <NCheckbox v-model:checked="createKey">
            {{ t('firstKey') }}
          </NCheckbox>
          <label class="phase-d__field">
            {{ t('description') }}
            <NInput
              v-model:value="description"
              :input-props="{ 'aria-label': t('description') }"
            />
          </label>

          <div
            class="phase-d__panel phase-d__stack"
            style="margin-top: 16px; padding: 14px"
          >
            <strong style="font-size: 14px">{{ t('summaryTitle') }}</strong>
            <div style="display: grid; grid-template-columns: 140px 1fr; gap: 8px; font-size: 13px">
              <span class="phase-d__muted">{{ t('summaryUser') }}:</span>
              <strong>{{ name }}</strong>

              <span class="phase-d__muted">{{ t('summaryPermission') }}:</span>
              <span>
                <NTag
                  size="small"
                  type="info"
                >
                  {{ permission }}
                </NTag>
              </span>

              <span class="phase-d__muted">{{ t('summaryBuckets') }}:</span>
              <span><code>{{ effectiveBucketsDisplay }}</code></span>
            </div>
          </div>
        </template>

        <Callout
          v-if="error"
          tone="danger"
        >
          {{ error }}
        </Callout>
        <div class="phase-d__actions">
          <NButton
            v-if="step > 1"
            :disabled="busy"
            @click="step--"
          >
            {{ t('back') }}
          </NButton>
          <NButton
            type="primary"
            attr-type="submit"
            :loading="busy"
            :disabled="
              !store.dynamicEnabled || store.isLoading || store.hasError
            "
          >
            {{ t(step === 3 ? 'createUser' : 'next') }}
          </NButton>
        </div>
      </form>
    </SectionCard>
    <ProxySecretModal
      :secret="secret"
      kind="created"
      @close="finish"
    />
  </div>
</template>

<style lang="scss" src="@/components/tutti/users/phaseD.scss"></style>
