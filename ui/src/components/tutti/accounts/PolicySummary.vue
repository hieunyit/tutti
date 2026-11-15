<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  /** One sentence that previews what the user will be able to do after saving. */
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nPolicy from './i18nPolicy';
  import { AccessPolicyHelper } from '@/utils/helpers/AccessPolicyHelper';
  import type { TuttiAccessPolicyForm } from '@/utils/types/tutti';

  const props = defineProps<{ form: TuttiAccessPolicyForm }>();
  const { t } = useI18n({ messages: i18nPolicy });

  const permission = computed(() =>
    t(
      {
        'read-write': 'permissionReadWrite',
        'read-only': 'permissionReadOnly',
        'write-only': 'permissionWriteOnly',
      }[props.form.permission] ?? 'permissionReadWrite',
    ),
  );
  const text = computed(() =>
    AccessPolicyHelper.isAllBuckets(props.form.buckets)
      ? t('summaryAll', { user: props.form.user, permission: permission.value })
      : t('summaryBuckets', {
          user: props.form.user,
          permission: permission.value,
          buckets: props.form.buckets.join(', '),
        }),
  );
</script>

<template>
  <p
    class="policy-summary"
    role="status"
  >
    {{ text }}
  </p>
</template>

<style scoped lang="scss">
  .policy-summary {
    margin: 0;
    padding: var(--tt-space-3);
    border: 1px solid var(--tt-border);
    border-radius: var(--tt-radius-control);
    background: var(--tt-subtle);
    font-size: var(--tt-text-sm);
    color: var(--tt-text-2);
    overflow-wrap: anywhere;
  }
</style>
