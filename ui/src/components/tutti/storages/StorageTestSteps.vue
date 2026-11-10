<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nStorages from './i18nStorages';
  import Callout from '@/components/ui/Callout.vue';
  import StatusTag from '@/components/ui/StatusTag.vue';
  import { formatDateTime } from '@/utils/helpers/DateTimeHelper';
  import { StorageTestHelper } from '@/utils/helpers/StorageTestHelper';
  import type { StorageTestResult } from '@/utils/types/runtimeStorage';

  const props = defineProps<{ result: StorageTestResult | null }>();
  const { t, locale } = useI18n({ messages: i18nStorages });
  const steps = computed(() => StorageTestHelper.orderedSteps(props.result));
  const counts = computed(() => StorageTestHelper.counts(props.result));
  const tested = computed(() =>
    props.result?.testedAt
      ? formatDateTime(props.result.testedAt, locale.value)
      : '',
  );
  const duration = computed(
    () => StorageTestHelper.latency(props.result?.durationMs) ?? 0,
  );
  const latency = (value: number | string | undefined) => {
    const ms = StorageTestHelper.latency(value);

    return ms === null ? '–' : `${ms} ms`;
  };
</script>

<template>
  <div
    v-if="result"
    class="test-steps"
  >
    <p
      class="test-steps__summary"
      role="status"
    >
      <StatusTag
        :variant="
          !result.ok || counts.fail > 0
            ? 'danger'
            : result.hasWarnings || counts.warn > 0
              ? 'warning'
              : 'success'
        "
        >{{
          !result.ok || counts.fail > 0
            ? t('status_fail')
            : result.hasWarnings || counts.warn > 0
              ? t('status_warn')
              : t('status_pass')
        }}</StatusTag
      >
      {{ t('summary', { ...counts, ms: duration }) }}
      <small v-if="result.kind === 'periodic'">{{ t('kindPeriodic') }}</small>
      <small v-if="tested">{{ t('testedAt', { time: tested }) }}</small>
    </p>
    <Callout
      v-if="result.leftovers?.length"
      tone="warning"
      >{{ t('leftovers', { list: result.leftovers.join(', ') }) }}</Callout
    >
    <div class="tt-table-wrap">
      <table class="tt-table test-steps__table">
        <thead>
          <tr>
            <th>{{ t('stepNo') }}</th>
            <th>{{ t('stepName') }}</th>
            <th>{{ t('stepStatus') }}</th>
            <th>{{ t('stepCall') }}</th>
            <th>{{ t('stepLatency') }}</th>
            <th>{{ t('stepDetail') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="step in steps"
            :key="step.id || step.number"
            :class="`test-steps__row--${StorageTestHelper.stepMeta(step.status).status}`"
          >
            <td :data-label="t('stepNo')">{{ step.number }}</td>
            <td :data-label="t('stepName')">{{ step.name }}</td>
            <td :data-label="t('stepStatus')">
              <StatusTag
                :variant="StorageTestHelper.stepMeta(step.status).variant"
                no-dot
                ><span aria-hidden="true">{{
                  StorageTestHelper.stepMeta(step.status).icon
                }}</span>
                {{
                  t(`status_${StorageTestHelper.stepMeta(step.status).status}`)
                }}</StatusTag
              >
            </td>
            <td :data-label="t('stepCall')">
              <code v-if="step.call">{{ step.call }}</code
              ><span v-else>–</span>
            </td>
            <td :data-label="t('stepLatency')">
              {{ latency(step.latencyMs) }}
            </td>
            <td :data-label="t('stepDetail')">
              <span v-if="step.message">{{ step.message }}</span>
              <small v-if="step.missingPermission"
                >{{ t('missingPermission') }}:
                <code>{{ step.missingPermission }}</code></small
              >
              <small v-if="step.hint">{{ t('hint') }}: {{ step.hint }}</small>
              <span
                v-if="!step.message && !step.missingPermission && !step.hint"
                >–</span
              >
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
  <p
    v-else
    class="test-steps__empty"
  >
    {{ t('noSteps') }}
  </p>
</template>

<style scoped lang="scss">
  .test-steps {
    display: flex;
    flex-direction: column;
    gap: 12px;
    min-width: 0;

    &__summary {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      gap: 4px 10px;
      margin: 0;
      overflow-wrap: anywhere;
    }

    &__empty {
      color: var(--tt-text-3);
    }

    small {
      display: block;
      color: var(--tt-text-3);
      font-size: 12px;
      overflow-wrap: anywhere;
    }

    code {
      overflow-wrap: anywhere;
    }

    td {
      overflow-wrap: anywhere;
      white-space: normal;
      vertical-align: top;
    }

    td:nth-child(1) {
      white-space: nowrap;
    }

    td:nth-child(2) {
      min-width: 120px;
      overflow-wrap: normal;
    }

    td:nth-child(4) {
      min-width: 160px;
    }

    td:nth-child(6) {
      min-width: 220px;
    }

    &__row--fail td:nth-child(2) {
      color: var(--tt-danger);
      font-weight: 600;
    }
    @media (max-width: 760px) {
      &__table {
        display: block;

        thead {
          display: none;
        }

        tbody,
        tr {
          display: block;
        }

        tr {
          padding: 10px 0;
          border-bottom: 1px solid var(--tt-border);
        }

        td {
          display: grid;
          grid-template-columns: minmax(70px, 1fr) minmax(0, 3fr);
          gap: 4px 8px;
          padding: 4px 0;
          white-space: normal;
          border: 0;

          &::before {
            content: attr(data-label);
            color: var(--tt-text-3);
            font-size: 12px;
          }

          > small {
            grid-column: 2;
          }
        }
      }
    }
  }
</style>
