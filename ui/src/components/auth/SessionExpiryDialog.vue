<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  /**
   * Session warnings: a countdown dialog when the idle deadline is close
   * (Continue / Sign out) and a banner with the local end time when the
   * absolute cap is close (nothing to extend there).
   */
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NButton, NModal } from 'naive-ui';
  import i18nSession from './i18nSession';
  import { Callout } from '@/components/kit';

  const props = defineProps<{
    warning: 'none' | 'idle' | 'max';
    remainingIdleMs: number;
    remainingMaxMs: number;
  }>();

  const emit = defineEmits<{
    (e: 'continue'): void;
    (e: 'signOut'): void;
  }>();

  const { t } = useI18n({ messages: i18nSession });

  const pad = (n: number) => String(n).padStart(2, '0');

  const countdown = computed(() => {
    const total = Math.max(0, Math.ceil(props.remainingIdleMs / 1000));

    return `${Math.floor(total / 60)}:${pad(total % 60)}`;
  });

  /** local wall-clock time of the cap, measured from the server-corrected remainder */
  const maxTime = computed(() => {
    const end = new Date(Date.now() + Math.max(0, props.remainingMaxMs));

    return `${pad(end.getHours())}:${pad(end.getMinutes())}`;
  });
</script>

<template>
  <NModal
    :show="warning === 'idle'"
    preset="card"
    role="alertdialog"
    :title="t('idleTitle')"
    :closable="false"
    :mask-closable="false"
    :close-on-esc="false"
    :style="{ width: '420px', maxWidth: 'calc(100vw - 32px)' }"
  >
    <p
      class="session-expiry__text"
      data-session-idle-text
    >
      {{ t('idleText', { time: countdown }) }}
    </p>
    <template #footer>
      <div class="session-expiry__actions">
        <NButton
          data-session-sign-out
          @click="emit('signOut')"
        >
          {{ t('signOut') }}
        </NButton>
        <NButton
          type="primary"
          data-session-continue
          @click="emit('continue')"
        >
          {{ t('continue') }}
        </NButton>
      </div>
    </template>
  </NModal>

  <div
    v-if="warning === 'max'"
    class="session-expiry__banner"
    role="status"
    data-session-max
  >
    <Callout tone="warning">{{ t('maxText', { time: maxTime }) }}</Callout>
  </div>
</template>

<style lang="scss" scoped>
  .session-expiry {
    &__text {
      margin: 0;
      color: var(--tt-text-2);
      line-height: 20px;
    }

    &__actions {
      display: flex;
      justify-content: flex-end;
      flex-wrap: wrap;
      gap: var(--tt-space-2);
    }

    &__banner {
      position: fixed;
      right: var(--tt-space-4);
      bottom: var(--tt-space-4);
      z-index: 3000;
      max-width: min(420px, calc(100vw - 32px));
    }
  }
</style>
