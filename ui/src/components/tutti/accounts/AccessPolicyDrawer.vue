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
  import {
    NAlert,
    NButton,
    NCheckbox,
    NDrawer,
    NDrawerContent,
    NForm,
    NFormItem,
    NInput,
    NRadio,
    NSelect,
    NSpace,
    NTag,
  } from 'naive-ui';
  import { storeToRefs } from 'pinia';
  import { computed, ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nAccounts from './i18nAccounts';
  import { useTuttiAccessPolicyStore } from '@/stores/tuttiAccessPolicyStore';
  import { useTuttiProtectionStore } from '@/stores/tuttiProtectionStore';
  import { AccessPolicyHelper } from '@/utils/helpers/AccessPolicyHelper';
  import { ProtectionGroupHelper } from '@/utils/helpers/ProtectionGroupHelper';
  import {
    BucketDiscoveryHelper,
    type StorageBucketEntry,
  } from '@/utils/helpers/BucketDiscoveryHelper';
  import {
    TuttiAccessPermission,
    type TuttiAccessPolicyForm,
    type TuttiAccessPolicyValidationError,
    type TuttiProxyAccount,
  } from '@/utils/types/tutti';

  interface GroupBucketItem {
    bucket: string;
    groupId: string;
  }

  const props = defineProps<{
    show: boolean;
    account: TuttiProxyAccount | null;
  }>();

  const emit = defineEmits<{
    (e: 'update:show', value: boolean): void;
  }>();

  const { t } = useI18n({ messages: i18nAccounts });
  const store = useTuttiAccessPolicyStore();
  const protectionStore = useTuttiProtectionStore();
  const { isSaving, saveError } = storeToRefs(store);

  const form = ref<TuttiAccessPolicyForm>(AccessPolicyHelper.policyToForm(''));
  const grantMode = ref<'all' | 'groups' | 'custom'>('all');
  const selectedGroupIds = ref<string[]>([]);
  const selectedGroupBuckets = ref<string[]>([]);
  const customExtraBuckets = ref<string[]>([]);
  const clusterBuckets = ref<string[]>([]);
  const discoveredStorageBuckets = ref<StorageBucketEntry[]>([]);
  const extraPatternInput = ref('');
  const newPattern = ref('');
  const showErrors = ref(false);

  const permissions = computed(() => [
    {
      value: TuttiAccessPermission.READ_WRITE,
      label: t('permissionReadWrite'),
      hint: t('permissionReadWriteHint'),
    },
    {
      value: TuttiAccessPermission.READ_ONLY,
      label: t('permissionReadOnly'),
      hint: t('permissionReadOnlyHint'),
    },
    {
      value: TuttiAccessPermission.WRITE_ONLY,
      label: t('permissionWriteOnly'),
      hint: t('permissionWriteOnlyHint'),
    },
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
      clusterBuckets.value = await BucketDiscoveryHelper.discoverBuckets(
        props.account?.user,
      );
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
          props.account?.user,
        );
    } catch {
      discoveredStorageBuckets.value = [];
    }
  }

  function addSuggestedBucket(b: string) {
    if (!form.value.buckets.includes(b)) {
      form.value.buckets.push(b);
    }
  }

  function updateFormBucketsFromGroups() {
    if (
      storageScopeGroups.value.length > 0 &&
      selectedGroupBuckets.value.length === 0 &&
      customExtraBuckets.value.length === 0
    ) {
      form.value.buckets = ['*'];

      return;
    }

    const combined = [
      ...selectedGroupBuckets.value,
      ...customExtraBuckets.value,
    ];

    form.value.buckets = combined.length > 0 ? combined : [];
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
    updateFormBucketsFromGroups();
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

    updateFormBucketsFromGroups();
  }

  function selectAllGroupBuckets() {
    selectedGroupBuckets.value = availableGroupBuckets.value.map(
      (b) => b.bucket,
    );
    updateFormBucketsFromGroups();
  }

  function deselectAllGroupBuckets() {
    selectedGroupBuckets.value = [];
    updateFormBucketsFromGroups();
  }

  function addExtraPattern() {
    const pat = extraPatternInput.value.trim();

    if (!pat) return;

    const err = AccessPolicyHelper.validatePattern(pat);

    if (err) return;

    if (!customExtraBuckets.value.includes(pat)) {
      customExtraBuckets.value.push(pat);
    }

    extraPatternInput.value = '';
    updateFormBucketsFromGroups();
  }

  function removeExtraPattern(pattern: string) {
    customExtraBuckets.value = customExtraBuckets.value.filter(
      (p) => p !== pattern,
    );
    updateFormBucketsFromGroups();
  }

  watch(
    () => props.show,
    async (show) => {
      if (!show || !props.account) return;

      if (!protectionStore.isLoaded && !protectionStore.isLoading) {
        protectionStore.refresh();
      }

      loadClusterBuckets();

      form.value = AccessPolicyHelper.policyToForm(
        props.account.user,
        props.account.policy,
      );
      newPattern.value = '';
      extraPatternInput.value = '';
      showErrors.value = false;
      store.saveError = null;

      if (
        form.value.buckets.length === 0 ||
        form.value.buckets.includes('*')
      ) {
        grantMode.value = 'all';
        selectedGroupIds.value = [];
        selectedGroupBuckets.value = [];
        customExtraBuckets.value = [];
      } else {
        const matchingGroups = protectionStore.groups.filter((g) =>
          (g.buckets ?? []).some((b) => form.value.buckets.includes(b)),
        );

        if (matchingGroups.length > 0) {
          grantMode.value = 'groups';
          selectedGroupIds.value = matchingGroups.map((g) => g.id);
          await discoverBucketsForStorageGroups();

          const allGroupBucketsSet = new Set(
            matchingGroups.flatMap((g) => g.buckets ?? []),
          );

          for (const entry of discoveredStorageBuckets.value) {
            allGroupBucketsSet.add(entry.bucket);
          }

          selectedGroupBuckets.value = form.value.buckets.filter((b) =>
            allGroupBucketsSet.has(b),
          );
          customExtraBuckets.value = form.value.buckets.filter(
            (b) => !allGroupBucketsSet.has(b),
          );
        } else {
          grantMode.value = 'custom';
          selectedGroupIds.value = [];
          selectedGroupBuckets.value = [];
          customExtraBuckets.value = [];
        }
      }
    },
  );

  /** the not-yet-added input counts too so a typo is reported before saving */
  const patternError = computed<TuttiAccessPolicyValidationError | null>(() =>
    newPattern.value === ''
      ? null
      : AccessPolicyHelper.validatePattern(newPattern.value),
  );

  const errors = computed(() => AccessPolicyHelper.validate(form.value));

  const listErrors = computed(() => {
    const codes = errors.value.filter((e) => e.startsWith('PATTERN_'));
    const all = patternError.value ? [...codes, patternError.value] : codes;

    return showErrors.value || patternError.value
      ? [...new Set(all)].map((e) => t(e))
      : [];
  });

  function addPattern() {
    if (newPattern.value === '' || patternError.value) return;

    form.value.buckets = AccessPolicyHelper.addPattern(
      form.value.buckets,
      newPattern.value,
    );
    newPattern.value = '';
  }

  async function submit() {
    showErrors.value = true;

    // a pending, valid pattern is added implicitly
    if (grantMode.value === 'custom' && newPattern.value !== '') {
      if (patternError.value) return;

      addPattern();
    }

    if (grantMode.value === 'groups' && extraPatternInput.value.trim() !== '') {
      addExtraPattern();
    }

    if (AccessPolicyHelper.validate(form.value).length > 0) return;

    if (await store.savePolicy(form.value)) {
      emit('update:show', false);
    }
  }
</script>

<template>
  <NDrawer
    :show="show"
    width="min(540px, 100vw)"
    @update:show="emit('update:show', $event)"
  >
    <NDrawerContent
      :title="account?.hasPolicy ? t('editPolicy') : t('createPolicy')"
      closable
    >
      <NForm label-placement="top">
        <NFormItem :label="t('fieldUser')">
          <NInput
            :value="form.user"
            disabled
          />
        </NFormItem>

        <NFormItem :label="t('fieldPermission')">
          <NSpace
            vertical
            size="small"
          >
            <NRadio
              v-for="permission in permissions"
              :key="permission.value"
              :checked="form.permission === permission.value"
              :value="permission.value"
              @update:checked="form.permission = permission.value"
            >
              <strong>{{ permission.label }}</strong>
              <div class="policy-drawer__hint">{{ permission.hint }}</div>
            </NRadio>
          </NSpace>
        </NFormItem>

        <NFormItem :label="t('grantModeLabel')">
          <NSpace
            vertical
            size="small"
            style="width: 100%"
          >
            <NRadio
              :checked="grantMode === 'all'"
              value="all"
              @update:checked="
                grantMode = 'all';
                form.buckets = ['*'];
              "
            >
              <strong>{{ t('grantModeAll') }}</strong>
              <div class="policy-drawer__hint">{{ t('grantModeAllHint') }}</div>
            </NRadio>
            <NRadio
              :checked="grantMode === 'groups'"
              value="groups"
              @update:checked="
                grantMode = 'groups';
                updateFormBucketsFromGroups();
              "
            >
              <strong>{{ t('grantModeGroups') }}</strong>
              <div class="policy-drawer__hint">{{ t('grantModeGroupsHint') }}</div>
            </NRadio>
            <NRadio
              :checked="grantMode === 'custom'"
              value="custom"
              @update:checked="
                grantMode = 'custom';
                if (form.buckets.length === 1 && form.buckets[0] === '*') form.buckets = [];
              "
            >
              <strong>{{ t('grantModeCustom') }}</strong>
              <div class="policy-drawer__hint">{{ t('grantModeCustomHint') }}</div>
            </NRadio>
          </NSpace>
        </NFormItem>

        <template v-if="grantMode === 'groups'">
          <NFormItem :label="t('selectProtectionGroups')">
            <NSelect
              :value="selectedGroupIds"
              multiple
              filterable
              :options="protectionGroupOptions"
              :placeholder="t('selectProtectionGroupsPlaceholder')"
              @update:value="onGroupsChange"
            />
          </NFormItem>

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

          <NFormItem
            v-if="selectedGroupIds.length > 0"
            :label="t('groupBucketsLabel')"
            :feedback="t('groupBucketsHint')"
          >
            <div class="policy-drawer__buckets-box">
              <div
                v-if="availableGroupBuckets.length > 0"
                class="policy-drawer__buckets-toolbar"
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
                class="policy-drawer__bucket-grid"
              >
                <NCheckbox
                  v-for="item in availableGroupBuckets"
                  :key="item.bucket"
                  :checked="selectedGroupBuckets.includes(item.bucket)"
                  @update:checked="toggleGroupBucket(item.bucket, $event)"
                >
                  <span class="policy-drawer__bucket-name">{{ item.bucket }}</span>
                  <span class="policy-drawer__bucket-grp">({{ item.groupId }})</span>
                </NCheckbox>
              </div>
              <p
                v-else
                class="policy-drawer__hint"
              >
                {{ t('noBucketsInSelectedGroups') }}
              </p>
            </div>
          </NFormItem>

          <NFormItem
            v-if="selectedGroupIds.length > 0"
            :label="t('additionalPatternsLabel')"
            :feedback="t('additionalPatternsHint')"
          >
            <div class="policy-drawer__patterns">
              <div
                v-for="pattern in customExtraBuckets"
                :key="pattern"
                class="policy-drawer__pattern"
              >
                <NTag round>{{ pattern }}</NTag>
                <NButton
                  size="tiny"
                  :aria-label="t('removePattern')"
                  @click="removeExtraPattern(pattern)"
                >
                  ✕
                </NButton>
              </div>
              <div class="policy-drawer__add">
                <NInput
                  v-model:value="extraPatternInput"
                  :placeholder="t('bucketPlaceholder')"
                  @keydown.enter.prevent="addExtraPattern"
                />
                <NButton
                  :disabled="!extraPatternInput.trim()"
                  @click="addExtraPattern"
                >
                  {{ t('addPattern') }}
                </NButton>
              </div>
            </div>
          </NFormItem>

          <p
            v-if="selectedGroupIds.length === 0"
            class="policy-drawer__hint"
            style="margin-bottom: 16px"
          >
            {{ t('noGroupSelectedHint') }}
          </p>
        </template>

        <NFormItem
          v-if="grantMode === 'custom'"
          :label="t('fieldBuckets')"
          :validation-status="listErrors.length ? 'error' : undefined"
          :feedback="listErrors[0] ?? t('fieldBucketsHint')"
        >
          <div
            v-if="clusterBuckets.length > 0"
            class="policy-drawer__cluster-box"
          >
            <span class="policy-drawer__cluster-title">
              {{ t('clusterBucketsLabel') }}
            </span>
            <div class="policy-drawer__cluster-tags">
              <NTag
                v-for="b in clusterBuckets"
                :key="b"
                round
                clickable
                :type="form.buckets.includes(b) ? 'primary' : 'default'"
                @click="addSuggestedBucket(b)"
              >
                + {{ b }}
              </NTag>
            </div>
          </div>

          <div class="policy-drawer__patterns">
            <p
              v-if="form.buckets.length === 0"
              class="policy-drawer__hint"
            >
              {{ t('noPatterns') }}
            </p>
            <div
              v-for="pattern in form.buckets"
              :key="pattern"
              class="policy-drawer__pattern"
            >
              <NTag round>{{ pattern }}</NTag>
              <NButton
                size="tiny"
                :aria-label="t('removePattern')"
                @click="
                  form.buckets = AccessPolicyHelper.removePattern(
                    form.buckets,
                    pattern,
                  )
                "
              >
                ✕
              </NButton>
            </div>
            <div class="policy-drawer__add">
              <NInput
                v-model:value="newPattern"
                :placeholder="t('bucketPlaceholder')"
                @keydown.enter.prevent="addPattern"
              />
              <NButton
                :disabled="newPattern === '' || patternError !== null"
                @click="addPattern"
              >
                {{ t('addPattern') }}
              </NButton>
            </div>
          </div>
        </NFormItem>

        <NFormItem :label="t('fieldDescription')">
          <NInput
            v-model:value="form.description"
            type="textarea"
            :autosize="{ minRows: 2, maxRows: 4 }"
          />
        </NFormItem>
      </NForm>

      <NAlert
        v-if="saveError"
        type="error"
      >
        {{ saveError === 'unknown' ? t('saveUnknownError') : saveError }}
      </NAlert>

      <template #footer>
        <NButton
          class="policy-drawer__cancel"
          @click="emit('update:show', false)"
        >
          {{ t('cancel') }}
        </NButton>
        <NButton
          type="primary"
          :loading="isSaving"
          @click="submit"
        >
          {{ t('save') }}
        </NButton>
      </template>
    </NDrawerContent>
  </NDrawer>
</template>

<style lang="scss" scoped>
  .policy-drawer {
    &__hint {
      margin: 0;
      font-size: 12px;
      opacity: 0.7;
    }

    &__patterns {
      width: 100%;
    }

    &__pattern {
      display: flex;
      align-items: center;
      gap: 8px;
      margin-bottom: 8px;
    }

    &__add {
      display: flex;
      gap: 8px;
    }

    &__cancel {
      margin-right: 8px;
    }

    &__buckets-box {
      width: 100%;
      padding: 10px 12px;
      border: 1px solid var(--n-border-color, #e0e0e6);
      border-radius: 6px;
      background: var(--n-color-embedded, #fcfcfc);
    }

    &__buckets-toolbar {
      display: flex;
      gap: 8px;
      margin-bottom: 8px;
      padding-bottom: 6px;
      border-bottom: 1px dashed var(--n-border-color, #e0e0e6);
    }

    &__bucket-grid {
      display: flex;
      flex-direction: column;
      gap: 8px;
      max-height: 200px;
      overflow-y: auto;
    }

    &__bucket-name {
      font-weight: 500;
      margin-right: 4px;
    }

    &__cluster-box {
      width: 100%;
      margin-bottom: 12px;
      padding: 10px 12px;
      border: 1px dashed var(--tt-border-strong);
      border-radius: 6px;
      background: var(--tt-subtle);
    }

    &__cluster-title {
      display: block;
      margin-bottom: 6px;
      color: var(--tt-text-2);
      font-weight: 500;
      font-size: 12px;
    }

    &__cluster-tags {
      display: flex;
      flex-wrap: wrap;
      gap: 6px;
    }

    &__bucket-grp {
      font-size: 11px;
      opacity: 0.65;
    }
  }
</style>
