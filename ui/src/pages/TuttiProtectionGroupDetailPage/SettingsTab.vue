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
  import { computed, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import { Callout, ConfirmDialog, EMPTY_VALUE } from '@/components/kit';
  import i18nProtection from '@/components/tutti/protection/i18nProtectionUi';
  import { backendMessage } from '@/stores/tuttiProtectionStore';
  import type { ProtectionGroupActions } from '@/utils/composables/useProtectionGroupActions';
  import { ProtectionGroupHelper } from '@/utils/helpers/ProtectionGroupHelper';
  import type {
    ProtectionGroup,
    ProtectionGroupStatus,
  } from '@/utils/types/protection';

  const props = defineProps<{
    group: ProtectionGroup;
    detail: ProtectionGroupStatus | null;
    actions: Pick<ProtectionGroupActions, 'remove'>;
  }>();

  const emit = defineEmits<{
    (e: 'edit'): void;
    (e: 'removed'): void;
  }>();

  const { t } = useI18n({ messages: i18nProtection });

  const showDelete = ref(false);
  const busy = ref(false);
  const error = ref<string | null>(null);

  const facts = computed(() => {
    const group = props.group;
    const seconds = ProtectionGroupHelper.afterSeconds(group.after);

    return [
      [
        t('settingCopies'),
        t('copiesValue', {
          healthy: props.detail?.healthyCopies ?? EMPTY_VALUE,
          wanted: ProtectionGroupHelper.wantedCopies(group),
        }),
      ],
      [
        t('settingMode'),
        group.mode === 'auto' ? t('modeAuto') : t('modeManual'),
      ],
      [t('settingAfter'), t('settingSeconds', { value: seconds ?? 30 })],
      [t('settingLag'), String(group.maxEventLag ?? 0)],
      [t('settingSwitches'), String(group.maxSwitchesPerHour ?? 3)],
      [
        t('settingElectorate'),
        (group.electorate ?? []).join(', ') || EMPTY_VALUE,
      ],
    ];
  });

  function openDelete() {
    error.value = null;
    showDelete.value = true;
  }

  async function confirmDelete() {
    busy.value = true;
    error.value = null;

    try {
      await props.actions.remove();
      showDelete.value = false;
      emit('removed');
    } catch (e) {
      error.value = t('deleteFailed', {
        message: backendMessage(e) ?? 'unknown',
      });
    } finally {
      busy.value = false;
    }
  }
</script>

<template>
  <div class="settings-tab">
    <section class="settings-tab__section">
      <div class="settings-tab__head">
        <div>
          <h2 class="settings-tab__title">{{ t('editTitle') }}</h2>
          <p class="settings-tab__desc">{{ t('editDesc') }}</p>
        </div>
        <WriteGate>
          <NButton
            secondary
            :disabled="!ProtectionGroupHelper.canEdit(group)"
            @click="emit('edit')"
          >
            {{ t('editAction') }}
          </NButton>
        </WriteGate>
      </div>
      <dl class="settings-tab__facts">
        <div
          v-for="[label, value] in facts"
          :key="label"
        >
          <dt>{{ label }}</dt>
          <dd>{{ value }}</dd>
        </div>
      </dl>
    </section>

    <section class="settings-tab__section settings-tab__section--danger">
      <div class="settings-tab__head">
        <div>
          <h2 class="settings-tab__title">{{ t('dangerTitle') }}</h2>
          <p class="settings-tab__desc">{{ t('dangerDesc') }}</p>
        </div>
        <WriteGate>
          <NButton
            data-delete-group
            type="error"
            secondary
            :disabled="!ProtectionGroupHelper.canDelete(group)"
            @click="openDelete"
          >
            {{ t('deleteGroup') }}
          </NButton>
        </WriteGate>
      </div>
    </section>

    <ConfirmDialog
      :open="showDelete"
      :title="t('deleteTitle', { id: group.id })"
      :target-name="group.id"
      require-typing
      danger
      :loading="busy"
      :confirm-label="t('deleteConfirm')"
      @confirm="confirmDelete"
      @cancel="showDelete = false"
    >
      <p>{{ t('deleteText') }}</p>
      <Callout
        v-if="error"
        tone="danger"
      >
        {{ error }}
      </Callout>
    </ConfirmDialog>
  </div>
</template>

<style lang="scss" scoped>
  .settings-tab {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-4);

    &__section {
      padding: var(--tt-space-4);
      border: 1px solid var(--tt-border);
      border-radius: var(--tt-radius-card);
      background-color: var(--tt-surface);

      &--danger {
        border-color: var(--tt-danger-border);
      }
    }

    &__head {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      justify-content: space-between;
      gap: var(--tt-space-3);
    }

    &__title {
      margin: 0;
      font-size: var(--tt-text-md);
      font-weight: var(--tt-weight-strong);
    }

    &__desc {
      margin: var(--tt-space-1) 0 0;
      color: var(--tt-text-3);
      font-size: var(--tt-text-sm);
    }

    &__facts {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
      gap: var(--tt-space-4) var(--tt-space-5);
      margin: var(--tt-space-4) 0 0;

      dt {
        color: var(--tt-text-3);
        font-size: var(--tt-text-xs);
      }

      dd {
        margin: var(--tt-space-1) 0 0;
        font-weight: var(--tt-weight-medium);
        overflow-wrap: anywhere;
      }
    }
  }
</style>
