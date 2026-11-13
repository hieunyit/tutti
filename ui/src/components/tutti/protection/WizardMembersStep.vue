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
  import { NButton, NFormItem, NInputNumber, NSelect, NTag } from 'naive-ui';
  import { storeToRefs } from 'pinia';
  import { computed, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nProtection from './i18nProtectionUi';
  import { FormSection } from '@/components/kit';
  import { useTuttiProtectionStore } from '@/stores/tuttiProtectionStore';
  import { ProtectionGroupHelper } from '@/utils/helpers/ProtectionGroupHelper';
  import type {
    ProtectionFormError,
    ProtectionGroupForm,
  } from '@/utils/types/protection';

  defineProps<{
    errorText: (...codes: ProtectionFormError[]) => string | undefined;
  }>();

  const form = defineModel<ProtectionGroupForm>({ required: true });
  const { t } = useI18n({ messages: i18nProtection });
  const { storageNames } = storeToRefs(useTuttiProtectionStore());
  const selectedStorage = ref<string | null>(null);

  const storageOptions = computed(() =>
    storageNames.value
      .filter((name) => !form.value.members.includes(name))
      .map((name) => ({ label: name, value: name })),
  );

  function addMember() {
    if (!selectedStorage.value) return;

    form.value.members = ProtectionGroupHelper.addMember(
      form.value.members,
      selectedStorage.value,
    );
    selectedStorage.value = null;
  }

  function moveMember(index: number, dir: -1 | 1) {
    form.value.members = ProtectionGroupHelper.moveMember(
      form.value.members,
      index,
      dir,
    );
  }

  function removeMember(storage: string) {
    form.value.members = ProtectionGroupHelper.removeMember(
      form.value.members,
      storage,
    );
  }
</script>

<template>
  <FormSection
    :title="t('wizStepMembers')"
    :description="t('wizMembersDesc')"
  >
    <NFormItem
      :label="t('fieldMembers')"
      :validation-status="errorText('MEMBERS_MIN') ? 'error' : undefined"
      :feedback="errorText('MEMBERS_MIN') ?? t('fieldMembersHelp')"
    >
      <div class="pg-wizard__stack">
        <p
          v-if="form.members.length === 0"
          class="pg-wizard__hint"
        >
          {{ t('fieldMembersEmpty') }}
        </p>
        <div
          v-for="(member, index) in form.members"
          :key="member"
          class="pg-wizard__row"
        >
          <NTag round>{{ index + 1 }}</NTag>
          <span class="pg-wizard__member tt-mono">{{ member }}</span>
          <NTag
            v-if="index === 0"
            type="info"
            round
          >
            {{ t('primaryMark') }}
          </NTag>
          <NButton
            size="tiny"
            :disabled="index === 0"
            :aria-label="t('moveUp')"
            @click="moveMember(index, -1)"
          >
            ↑
          </NButton>
          <NButton
            size="tiny"
            :disabled="index === form.members.length - 1"
            :aria-label="t('moveDown')"
            @click="moveMember(index, 1)"
          >
            ↓
          </NButton>
          <NButton
            size="tiny"
            :aria-label="t('remove')"
            @click="removeMember(member)"
          >
            ✕
          </NButton>
        </div>
        <div class="pg-wizard__row">
          <NSelect
            v-model:value="selectedStorage"
            :options="storageOptions"
            :placeholder="t('fieldMembersAdd')"
            filterable
            tag
          />
          <NButton
            :disabled="!selectedStorage"
            @click="addMember"
          >
            {{ t('fieldMembersAdd') }}
          </NButton>
        </div>
      </div>
    </NFormItem>
    <NFormItem
      :label="t('fieldCopies')"
      :validation-status="errorText('COPIES_RANGE') ? 'error' : undefined"
      :feedback="errorText('COPIES_RANGE') ?? t('fieldCopiesHelp')"
    >
      <NInputNumber
        v-model:value="form.copies"
        :min="2"
        :max="Math.max(form.members.length, 2)"
        :precision="0"
      />
    </NFormItem>
  </FormSection>
</template>

<style lang="scss" scoped>
  .pg-wizard {
    &__stack {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-2);
      width: 100%;
    }

    &__row {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      gap: var(--tt-space-2);
      width: 100%;
    }

    &__member {
      flex: 1;
      min-width: 0;
      overflow-wrap: anywhere;
    }

    &__hint {
      margin: 0;
      color: var(--tt-text-3);
      font-size: var(--tt-text-sm);
    }
  }
</style>
