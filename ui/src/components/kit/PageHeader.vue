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
  /**
   * Page header: title (H1), an optional ONE-line description (only when it adds
   * what the title lacks), the primary action (slot `actions`) and a secondary
   * actions menu (`secondaryActions`, emits `action` with the key). A
   * description equal to the title is dropped, so it never restates it.
   */
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NButton, NDropdown } from 'naive-ui';
  import i18nKit from './i18nKit';
  import type { PageAction } from './types';

  const props = withDefaults(
    defineProps<{
      title: string;
      description?: string;
      secondaryActions?: PageAction[];
    }>(),
    { description: undefined, secondaryActions: () => [] },
  );

  const emit = defineEmits<{
    (e: 'action', key: string): void;
  }>();

  const { t } = useI18n({ messages: i18nKit });
  const text = computed(() => {
    const value = props.description?.trim();

    return value && value.toLowerCase() !== props.title.trim().toLowerCase()
      ? value
      : undefined;
  });
  const menuOptions = computed(() =>
    props.secondaryActions.map((action) => ({
      key: action.key,
      label: action.label,
      disabled: action.disabled,
      props: action.danger ? { class: 'kit-header__danger' } : undefined,
    })),
  );
</script>

<template>
  <div class="kit-header">
    <div
      v-if="$slots.back"
      class="kit-header__back"
    >
      <slot name="back" />
    </div>
    <div class="kit-header__row">
      <div class="kit-header__heading">
        <div class="kit-header__title-row">
          <h1 class="kit-header__title">{{ title }}</h1>
          <slot name="meta" />
        </div>
        <p
          v-if="text"
          class="kit-header__description"
        >
          {{ text }}
        </p>
      </div>
      <div
        v-if="$slots.actions || menuOptions.length > 0"
        class="kit-header__actions"
      >
        <NDropdown
          v-if="menuOptions.length > 0"
          trigger="click"
          placement="bottom-end"
          :options="menuOptions"
          @select="emit('action', String($event))"
        >
          <NButton
            class="kit-header__more"
            :aria-label="t('moreActions')"
            aria-haspopup="menu"
          >
            {{ t('moreActions') }}
          </NButton>
        </NDropdown>
        <slot name="actions" />
      </div>
    </div>
  </div>
</template>

<style lang="scss" scoped>
  .kit-header {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-2);

    &__row {
      display: flex;
      align-items: flex-end;
      flex-wrap: wrap;
      gap: var(--tt-space-3) var(--tt-space-4);
    }

    &__heading {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-1);
      flex: 1 1 320px;
      min-width: 0;
    }

    &__title-row {
      display: flex;
      align-items: center;
      flex-wrap: wrap;
      gap: var(--tt-space-2) var(--tt-space-3);
      min-width: 0;
    }

    &__title {
      margin: 0;
      font-size: var(--tt-text-2xl);
      line-height: 32px;
      font-weight: var(--tt-weight-strong);
      color: var(--tt-text);
      overflow-wrap: anywhere;
    }

    &__description {
      margin: 0;
      max-width: var(--tt-measure);
      font-size: var(--tt-text-md);
      line-height: 20px;
      color: var(--tt-text-2);
    }

    &__actions {
      display: flex;
      align-items: center;
      flex-wrap: wrap;
      gap: var(--tt-space-2);
      margin-left: auto;
    }
  }
</style>
