<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  import { NFormItem, NInput, NInputNumber, NSelect } from 'naive-ui';
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nProxies from './i18nProxies';
  import { FormSection } from '@/components/kit';
  import { useTuttiProxyFleetConfigStore } from '@/stores/tuttiProxyFleetConfigStore';

  type FieldKind = 'bool' | 'number' | 'text' | 'choice';

  interface FieldSpec {
    key: string;
    label: string;
    path: string[];
    kind: FieldKind;
    options?: string[];
    placeholder?: string;
    restart?: boolean;
  }

  interface SectionSpec {
    key: string;
    fields: FieldSpec[];
  }

  const props = defineProps<{ scope: string }>();
  const { t } = useI18n({ messages: i18nProxies });
  const store = useTuttiProxyFleetConfigStore();

  // the five sections of spec 4.4; one generic renderer instead of ~15 setters
  const sections: SectionSpec[] = [
    {
      key: 'secSync',
      fields: [
        {
          key: 'sync',
          label: 'sync_enabled',
          path: ['replication', 'sync', 'enabled'],
          kind: 'bool',
        },
        {
          key: 'oqf',
          label: 'onQuorumFailure',
          path: ['replication', 'sync', 'onQuorumFailure'],
          kind: 'choice',
          options: ['fail', 'accept'],
        },
        {
          key: 'oee',
          label: 'onEventError',
          path: ['replication', 'onEventError'],
          kind: 'choice',
          options: ['fail', 'log'],
        },
        {
          key: 'erb',
          label: 'eventRetryBudget',
          path: ['replication', 'eventRetryBudget'],
          kind: 'text',
          placeholder: '5m',
        },
      ],
    },
    {
      key: 'secHealth',
      fields: [
        {
          key: 'ft',
          label: 'failureThreshold',
          path: ['health', 'failureThreshold'],
          kind: 'number',
        },
        {
          key: 'od',
          label: 'openDuration',
          path: ['health', 'openDuration'],
          kind: 'text',
          placeholder: '30s',
        },
      ],
    },
    {
      key: 'secLease',
      fields: [
        {
          key: 'wl',
          label: 'writerLease',
          path: ['writerLease'],
          kind: 'text',
          placeholder: '30s',
          restart: true,
        },
        {
          key: 'wq',
          label: 'writeQuorum',
          path: ['replication', 'sync', 'writeQuorum'],
          kind: 'number',
        },
      ],
    },
    {
      key: 'secTimeouts',
      fields: [
        {
          key: 'st',
          label: 'syncTimeout',
          path: ['replication', 'sync', 'timeout'],
          kind: 'text',
          placeholder: '30s',
        },
        {
          key: 'rht',
          label: 'readHeaderTimeout',
          path: ['readHeaderTimeout'],
          kind: 'text',
          placeholder: '10s',
          restart: true,
        },
        {
          key: 'dd',
          label: 'drainDelay',
          path: ['drainDelay'],
          kind: 'text',
          placeholder: '5s',
        },
        {
          key: 'rms',
          label: 'readyMaxStaleness',
          path: ['readyMaxStaleness'],
          kind: 'text',
          placeholder: '30s',
        },
      ],
    },
    {
      key: 'secStorages',
      fields: [
        {
          key: 'fo',
          label: 'failoverEnabled',
          path: ['failover', 'enabled'],
          kind: 'bool',
        },
        {
          key: 'ci',
          label: 'checkInterval',
          path: ['failover', 'checkInterval'],
          kind: 'text',
          placeholder: '10s',
        },
        {
          key: 'fa',
          label: 'failoverAfter',
          path: ['failover', 'after'],
          kind: 'text',
          placeholder: '1m',
        },
        {
          key: 'mel',
          label: 'maxEventLag',
          path: ['failover', 'maxEventLag'],
          kind: 'number',
        },
        {
          key: 'rsc',
          label: 'requireSignedContentType',
          path: ['requireSignedContentType'],
          kind: 'bool',
        },
      ],
    },
  ];

  const boolOptions = computed(() => [
    { label: t('inherit'), value: '' },
    { label: t('on'), value: 'true' },
    { label: t('off'), value: 'false' },
  ]);

  function choiceOptions(values: string[]) {
    return [
      { label: t('inherit'), value: '' },
      ...values.map((value) => ({ label: value, value })),
    ];
  }

  function label(field: FieldSpec) {
    return field.restart
      ? `${t(field.label)} (${t('needsRestart')})`
      : t(field.label);
  }

  const value = (field: FieldSpec) => store.getField(props.scope, field.path);

  function set(field: FieldSpec, next: unknown) {
    const finite = typeof next !== 'number' || Number.isFinite(next);
    const clean =
      typeof next === 'string' && field.kind === 'text' ? next.trim() : next;

    store.setField(props.scope, field.path, finite ? clean : undefined);
  }

  function setBool(field: FieldSpec, next: string | null) {
    store.setField(props.scope, field.path, next ? next === 'true' : undefined);
  }
</script>

<template>
  <div class="scope-form">
    <FormSection
      v-for="section in sections"
      :key="section.key"
      :title="t(section.key)"
      :description="t(`${section.key}Hint`)"
    >
      <NFormItem
        v-for="field in section.fields"
        :key="field.key"
        :label="label(field)"
        label-placement="top"
      >
        <NSelect
          v-if="field.kind === 'bool'"
          :value="value(field) === undefined ? '' : String(value(field))"
          :options="boolOptions"
          @update:value="setBool(field, $event)"
        />
        <NSelect
          v-else-if="field.kind === 'choice'"
          :value="(value(field) as string | undefined) ?? ''"
          :options="choiceOptions(field.options ?? [])"
          @update:value="set(field, $event)"
        />
        <NInputNumber
          v-else-if="field.kind === 'number'"
          :value="(value(field) as number | undefined) ?? null"
          :min="0"
          :placeholder="t('inherit')"
          clearable
          style="width: 100%"
          @update:value="set(field, $event)"
        />
        <NInput
          v-else
          :value="(value(field) as string | undefined) ?? ''"
          :placeholder="field.placeholder ?? t('inherit')"
          clearable
          @update:value="set(field, $event)"
        />
      </NFormItem>
    </FormSection>
  </div>
</template>

<style scoped lang="scss">
  .scope-form {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-4);
  }
</style>
