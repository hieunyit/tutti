<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  /** Actions of one access key: Rotate, plus a menu with Disable/Enable and Delete. */
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NButton, NDropdown } from 'naive-ui';
  import i18nAccounts from './i18nAccounts';
  import { isDisabled, type KeyRow } from './accessKeyRows';
  import WriteGate from '@/components/auth/WriteGate.vue';

  const props = defineProps<{ row: KeyRow }>();
  const emit = defineEmits<{
    (e: 'rotate'): void;
    (e: 'toggle'): void;
    (e: 'delete'): void;
  }>();
  const { t } = useI18n({ messages: i18nAccounts });
  const options = computed(() => [
    {
      key: 'toggle',
      label: t(isDisabled(props.row) ? 'enableKey' : 'disableKey'),
    },
    { key: 'delete', label: t('deleteKey') },
  ]);

  function onSelect(key: string) {
    if (key === 'toggle') emit('toggle');
    else emit('delete');
  }
</script>

<template>
  <span
    v-if="!row.dynamicKey"
    class="key-menu__config"
    >{{ t('managedInConfig') }}</span
  >
  <WriteGate v-else>
    <span class="key-menu">
      <NButton
        size="small"
        @click="emit('rotate')"
        >{{ t('rotate') }}</NButton
      >
      <NDropdown
        trigger="click"
        :options="options"
        @select="onSelect"
      >
        <NButton
          size="small"
          quaternary
          :aria-label="t('moreActions', { key: row.accessKeyId })"
          >⋯</NButton
        >
      </NDropdown>
    </span>
  </WriteGate>
</template>

<style scoped lang="scss">
  .key-menu {
    display: inline-flex;
    gap: var(--tt-space-1);
    align-items: center;

    &__config {
      font-size: var(--tt-text-xs);
      color: var(--tt-text-3);
    }
  }
</style>
