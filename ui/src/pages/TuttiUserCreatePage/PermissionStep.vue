<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  import { computed, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import {
    NAlert,
    NButton,
    NCheckbox,
    NInput,
    NRadio,
    NSelect,
    NTag,
  } from 'naive-ui';
  import { FormSection } from '@/components/kit';
  import messages from '@/components/tutti/users/i18nAccess';
  import { useTuttiProtectionStore } from '@/stores/tuttiProtectionStore';
  import { ProtectionGroupHelper } from '@/utils/helpers/ProtectionGroupHelper';
  import {
    injectUserCreate,
    type GrantMode,
  } from '@/utils/composables/useUserCreate';

  const { t } = useI18n({ messages });
  const ctl = injectUserCreate();
  const { form } = ctl;
  const protection = useTuttiProtectionStore();
  const extraInput = ref('');
  const extraError = ref('');

  const permissionOptions = computed(() => [
    { label: t('readWrite'), value: 'read-write' },
    { label: t('readOnly'), value: 'read-only' },
    { label: t('writeOnly'), value: 'write-only' },
  ]);
  const modes = computed<{ value: GrantMode; label: string; hint: string }[]>(
    () => [
      { value: 'all', label: t('grantModeAll'), hint: t('grantModeAllHint') },
      {
        value: 'groups',
        label: t('grantModeGroups'),
        hint: t('grantModeGroupsHint'),
      },
      {
        value: 'custom',
        label: t('grantModeCustom'),
        hint: t('grantModeCustomHint'),
      },
    ],
  );
  const groupOptions = computed(() =>
    protection.groups.map((group) => {
      const scope = ProtectionGroupHelper.scopeKind(group);
      const details =
        scope === 'storage'
          ? `All buckets on ${group.primary || (group.members ?? []).join(', ')}`
          : (group.buckets ?? []).join(', ') || '-';

      return { label: `${group.id} (${details})`, value: group.id };
    }),
  );

  function selectMode(mode: GrantMode) {
    form.value.grantMode = mode;

    if (mode === 'all') form.value.buckets = '*';
    else if (mode === 'custom' && form.value.buckets === '*')
      form.value.buckets = '';
  }

  function addExtra() {
    const problem = ctl.addExtraPattern(extraInput.value);

    extraError.value = problem ? t(problem) : '';

    if (!problem) extraInput.value = '';
  }
</script>

<template>
  <div class="permission-step">
    <FormSection :title="t('permission')">
      <NSelect
        v-model:value="form.permission"
        class="permission-step__select"
        :options="permissionOptions"
        :aria-label="t('permission')"
      />
    </FormSection>

    <FormSection
      :title="t('grantModeLabel')"
      :error="
        ctl.permissionProblem.value &&
        form.grantMode !== 'groups' &&
        form.buckets !== ''
          ? t(ctl.permissionProblem.value)
          : undefined
      "
    >
      <div class="permission-step__modes">
        <NRadio
          v-for="mode in modes"
          :key="mode.value"
          :checked="form.grantMode === mode.value"
          @update:checked="selectMode(mode.value)"
        >
          <strong>{{ mode.label }}</strong>
          <p class="permission-step__hint">{{ mode.hint }}</p>
        </NRadio>
      </div>

      <template v-if="form.grantMode === 'groups'">
        <label class="permission-step__field">
          {{ t('selectProtectionGroups') }}
          <NSelect
            :value="form.selectedGroupIds"
            multiple
            filterable
            :options="groupOptions"
            :placeholder="t('selectProtectionGroupsPlaceholder')"
            @update:value="ctl.setGroups"
          />
        </label>
        <NAlert
          v-for="grp in ctl.storageGroups.value"
          :key="grp.id"
          type="info"
        >
          {{
            t('storageScopeAlert', {
              group: grp.id,
              storage: grp.primary || (grp.members ?? []).join(', '),
            })
          }}
        </NAlert>
        <p
          v-if="form.selectedGroupIds.length === 0"
          class="permission-step__hint"
        >
          {{ t('noGroupSelectedHint') }}
        </p>
        <div
          v-else
          class="permission-step__field"
        >
          <span>{{ t('groupBucketsLabel') }}</span>
          <p class="permission-step__hint">{{ t('groupBucketsHint') }}</p>
          <div v-if="ctl.availableGroupBuckets.value.length">
            <div class="permission-step__row">
              <NButton
                size="tiny"
                quaternary
                type="primary"
                @click="ctl.selectAllGroupBuckets(true)"
                >{{ t('selectAllBuckets') }}</NButton
              >
              <NButton
                size="tiny"
                quaternary
                @click="ctl.selectAllGroupBuckets(false)"
                >{{ t('deselectAllBuckets') }}</NButton
              >
            </div>
            <div class="permission-step__list">
              <NCheckbox
                v-for="item in ctl.availableGroupBuckets.value"
                :key="item.bucket"
                :checked="form.selectedGroupBuckets.includes(item.bucket)"
                @update:checked="ctl.setGroupBucket(item.bucket, $event)"
              >
                <strong>{{ item.bucket }}</strong>
                <span class="permission-step__hint"> ({{ item.groupId }})</span>
              </NCheckbox>
            </div>
          </div>
          <p
            v-else
            class="permission-step__hint"
          >
            {{ t('noBucketsInSelectedGroups') }}
          </p>

          <span>{{ t('additionalPatternsLabel') }}</span>
          <p class="permission-step__hint">{{ t('additionalPatternsHint') }}</p>
          <div class="permission-step__row">
            <NTag
              v-for="pattern in form.customExtraBuckets"
              :key="pattern"
              closable
              round
              @close="ctl.removeExtraPattern(pattern)"
              >{{ pattern }}</NTag
            >
          </div>
          <div class="permission-step__row">
            <NInput
              v-model:value="extraInput"
              :placeholder="t('bucketsHint')"
              :aria-label="t('additionalPatternsLabel')"
              @keydown.enter.prevent="addExtra"
            />
            <NButton
              :disabled="!extraInput.trim()"
              @click="addExtra"
              >{{ t('addPattern') }}</NButton
            >
          </div>
          <p
            v-if="extraError"
            class="permission-step__error"
            role="alert"
          >
            {{ extraError }}
          </p>
        </div>
      </template>

      <template v-if="form.grantMode === 'custom'">
        <div
          v-if="ctl.clusterBuckets.value.length"
          class="permission-step__field"
        >
          <span>{{ t('clusterBucketsLabel') }}</span>
          <p class="permission-step__hint">{{ t('clusterBucketsHint') }}</p>
          <div class="permission-step__row">
            <NTag
              v-for="bucket in ctl.clusterBuckets.value"
              :key="bucket"
              round
              checkable
              :checked="
                form.buckets.split(',').some((x) => x.trim() === bucket)
              "
              @click="ctl.addSuggestedBucket(bucket)"
              >+ {{ bucket }}</NTag
            >
          </div>
        </div>
        <label class="permission-step__field">
          {{ t('buckets') }}
          <NInput
            v-model:value="form.buckets"
            :input-props="{ 'aria-required': true }"
          />
          <span class="permission-step__hint">{{ t('bucketsHint') }}</span>
        </label>
      </template>
    </FormSection>
  </div>
</template>

<style lang="scss" scoped>
  .permission-step {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-5);

    &__select {
      max-width: 280px;
    }

    &__modes {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-2);
    }

    &__field {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-1);
      margin-top: var(--tt-space-3);
      font-size: var(--tt-text-sm);
    }

    &__hint {
      margin: 0;
      font-size: var(--tt-text-xs);
      color: var(--tt-text-3);
    }

    &__error {
      margin: 0;
      font-size: var(--tt-text-xs);
      color: var(--tt-danger);
    }

    &__row {
      display: flex;
      flex-wrap: wrap;
      gap: var(--tt-space-2);
      align-items: center;
    }

    &__list {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-2);
      max-height: 200px;
      margin-top: var(--tt-space-2);
      overflow-y: auto;
    }
  }
</style>
