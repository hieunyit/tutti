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
  import { NButton, NSelect } from 'naive-ui';
  import { computed, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import { Callout, ConfirmDialog, EMPTY_VALUE } from '@/components/kit';
  import i18nProtection from '@/components/tutti/protection/i18nProtectionUi';
  import { backendMessage } from '@/stores/tuttiProtectionStore';
  import type { ProtectionGroupActions } from '@/utils/composables/useProtectionGroupActions';
  import { ProtectionGroupHelper } from '@/utils/helpers/ProtectionGroupHelper';
  import type { ProtectionGroup } from '@/utils/types/protection';

  const props = defineProps<{
    group: ProtectionGroup;
    /** event lag per member storage, for the switchover warning */
    lagOf: (storage: string) => number;
    actions: Pick<ProtectionGroupActions, 'switchover' | 'abortSwitch'>;
  }>();

  const { t } = useI18n({ messages: i18nProtection });

  const showSwitch = ref(false);
  const showAbort = ref(false);
  const target = ref<string | null>(null);
  const busy = ref(false);
  const error = ref<string | null>(null);

  const candidates = computed(() =>
    ProtectionGroupHelper.switchTargets(props.group),
  );
  const options = computed(() =>
    candidates.value.map((storage) => ({ label: storage, value: storage })),
  );
  const targetLag = computed(() =>
    target.value ? props.lagOf(target.value) : 0,
  );

  function openSwitch() {
    target.value = candidates.value[0] ?? null;
    error.value = null;
    showSwitch.value = true;
  }

  function openAbort() {
    error.value = null;
    showAbort.value = true;
  }

  async function run(
    action: () => Promise<void>,
    failKey: string,
    close: () => void,
  ) {
    busy.value = true;
    error.value = null;

    try {
      await action();
      close();
    } catch (e) {
      error.value = t(failKey, { message: backendMessage(e) ?? 'unknown' });
    } finally {
      busy.value = false;
    }
  }

  function confirmSwitch() {
    const chosen = target.value;

    if (!chosen) return;

    run(
      () => props.actions.switchover(chosen),
      'switchFailed',
      () => (showSwitch.value = false),
    );
  }

  function confirmAbort() {
    run(
      () => props.actions.abortSwitch(),
      'abortFailed',
      () => (showAbort.value = false),
    );
  }
</script>

<template>
  <WriteGate>
    <div class="group-actions">
      <NButton
        v-if="ProtectionGroupHelper.canAbort(group)"
        type="warning"
        secondary
        @click="openAbort"
      >
        {{ t('cancelSwitch') }}
      </NButton>
      <NButton
        type="primary"
        :disabled="!ProtectionGroupHelper.canSwitch(group)"
        @click="openSwitch"
      >
        {{ group.mode === 'auto' ? t('failoverNow') : t('switchover') }}
      </NButton>
    </div>
  </WriteGate>

  <ConfirmDialog
    :open="showSwitch"
    :title="t('switchTitle', { id: group.id })"
    :target-name="group.id"
    danger
    :loading="busy"
    :confirm-label="t('switchConfirm', { target: target || '…' })"
    @confirm="confirmSwitch"
    @cancel="showSwitch = false"
  >
    <Callout
      v-if="candidates.length === 0"
      tone="warning"
    >
      {{ t('switchNoTargets') }}
    </Callout>
    <template v-else>
      <label class="group-actions__label">{{ t('switchPickTarget') }}</label>
      <NSelect
        v-model:value="target"
        :options="options"
        :placeholder="t('switchPickPlaceholder')"
        :disabled="busy"
      />
      <p
        v-if="target"
        class="group-actions__text"
      >
        {{
          t('switchConsequence', {
            target,
            primary: group.primary || EMPTY_VALUE,
          })
        }}
      </p>
      <Callout
        v-if="target && targetLag > 0"
        tone="warning"
      >
        {{ t('switchConsequenceLag', { target, lag: targetLag }) }}
      </Callout>
    </template>
    <Callout
      v-if="error"
      tone="danger"
    >
      {{ error }}
    </Callout>
  </ConfirmDialog>

  <ConfirmDialog
    :open="showAbort"
    :title="t('abortTitle')"
    :target-name="group.id"
    danger
    :loading="busy"
    :confirm-label="t('abortConfirm')"
    @confirm="confirmAbort"
    @cancel="showAbort = false"
  >
    <p class="group-actions__text">
      {{
        t('abortText', {
          target: group.target || EMPTY_VALUE,
          primary: group.primary || EMPTY_VALUE,
        })
      }}
    </p>
    <Callout
      v-if="error"
      tone="danger"
    >
      {{ error }}
    </Callout>
  </ConfirmDialog>
</template>

<style lang="scss" scoped>
  .group-actions {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--tt-space-2);

    &__label {
      display: block;
      margin-bottom: var(--tt-space-1);
      font-size: var(--tt-text-sm);
      color: var(--tt-text-2);
    }

    &__text {
      margin: var(--tt-space-2) 0;
      font-size: var(--tt-text-md);
      line-height: 20px;
      color: var(--tt-text-2);
    }
  }
</style>
