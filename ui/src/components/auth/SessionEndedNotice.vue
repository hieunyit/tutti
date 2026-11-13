<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  /** Login page message after the session ended (`?reason=idle|max`). */
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { useRoute } from 'vue-router';
  import i18nSession from './i18nSession';
  import { Callout } from '@/components/kit';

  const { t } = useI18n({ messages: i18nSession });
  const route = useRoute();

  const message = computed(() => {
    const reason = [route.query.reason].flat()[0];

    if (reason === 'idle') return t('endedIdle');

    return reason === 'max' ? t('endedMax') : '';
  });
</script>

<template>
  <Callout
    v-if="message"
    tone="warning"
    data-session-ended
  >
    {{ message }}
  </Callout>
</template>
