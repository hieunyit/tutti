<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  import { NButton } from 'naive-ui';
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import StorageTestSteps from './StorageTestSteps.vue';
  import i18nStorages from './i18nStorages';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import { Callout, SectionCard } from '@/components/kit';
  import StatusTag from '@/components/ui/StatusTag.vue';
  import { formatDateTime } from '@/utils/helpers/DateTimeHelper';
  import type {
    RuntimeStorage,
    StorageTestResult,
  } from '@/utils/types/runtimeStorage';
  import { RouteName } from '@/utils/types/router';

  const props = defineProps<{
    storage: RuntimeStorage;
    isConfig: boolean;
    busy: boolean;
    testing: boolean;
    shownTest: StorageTestResult | null;
  }>();
  const emit = defineEmits<{
    (e: 'rotate', mode: 'rotate' | 'set'): void;
    (e: 'test'): void;
  }>();
  const { t, locale } = useI18n({ messages: i18nStorages });
  const account = computed(() => props.storage.connectionAccount);
  const hasAccount = computed(() => account.value?.configured === true);
  const time = (value?: string) => formatDateTime(value, locale.value);
</script>

<template>
  <div class="storage-account">
    <SectionCard :title="t('connectionAccount')">
      <template v-if="hasAccount">
        <dl class="storage-account__facts">
          <div>
            <dt>{{ t('accessKey') }}</dt>
            <dd>
              <code>{{ account?.accessKeyId }}</code>
            </dd>
          </div>
          <div>
            <dt>{{ t('accountSource') }}</dt>
            <dd>
              <StatusTag
                no-dot
                :label="
                  account?.source === 'config' ? t('config') : t('managed')
                "
              />
            </dd>
          </div>
          <div v-if="account?.rotatedAt">
            <dt>{{ t('rotatedAt') }}</dt>
            <dd>{{ time(account.rotatedAt) }}</dd>
          </div>
          <div v-if="account?.previousAccessKeyId">
            <dt>{{ t('previousKey') }}</dt>
            <dd>
              <code>{{ account.previousAccessKeyId }}</code>
              <small v-if="account.safeToRevokePreviousAfter">{{
                Date.parse(account.safeToRevokePreviousAfter) > Date.now()
                  ? t('safeToRevoke', {
                      time: time(account.safeToRevokePreviousAfter),
                    })
                  : t('revokeNow')
              }}</small>
            </dd>
          </div>
        </dl>
        <p
          v-if="account?.source === 'config'"
          class="storage-account__sub"
        >
          {{ t('configAccountHint') }}
        </p>
      </template>
      <template v-else>
        <p>
          <StatusTag variant="warning">{{ t('legacy') }}</StatusTag>
        </p>
        <Callout
          v-if="storage.legacyCredentials !== false"
          tone="warning"
          >{{ t('legacyHint') }}</Callout
        >
      </template>
      <div
        v-if="!isConfig"
        class="storage-account__actions"
      >
        <WriteGate
          ><NButton
            :disabled="busy"
            @click="emit('rotate', hasAccount ? 'rotate' : 'set')"
            >{{ hasAccount ? t('rotate') : t('setAccount') }}</NButton
          ></WriteGate
        >
      </div>
      <RouterLink
        class="storage-account__link"
        :to="{
          name: RouteName.TUTTI_DOCS,
          params: { slug: 'storage-accounts' },
        }"
        >{{ t('guideLink') }}</RouterLink
      >
    </SectionCard>
    <SectionCard :title="t('lastTest')">
      <p
        v-if="storage.lastForcedSaveBy"
        class="storage-account__sub"
      >
        {{
          t('lastForced', {
            by: storage.lastForcedSaveBy,
            time: time(storage.lastForcedSaveAt),
          })
        }}
      </p>
      <StorageTestSteps :result="shownTest" />
      <div class="storage-account__actions">
        <WriteGate
          ><NButton
            :loading="testing"
            :disabled="!hasAccount || busy"
            @click="emit('test')"
            >{{ t('testNow') }}</NButton
          ></WriteGate
        >
      </div>
    </SectionCard>
  </div>
</template>

<style scoped lang="scss">
  .storage-account {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-4);

    &__actions {
      display: flex;
      flex-wrap: wrap;
      gap: var(--tt-space-2);
      justify-content: flex-end;
      margin-top: var(--tt-space-4);
    }

    &__sub {
      color: var(--tt-text-3);
      font-size: var(--tt-text-sm);
    }

    &__facts {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
      gap: var(--tt-space-3) var(--tt-space-4);
      margin: 0;

      dt {
        color: var(--tt-text-3);
        font-size: var(--tt-text-xs);
      }

      dd {
        margin: 2px 0 0;
        overflow-wrap: anywhere;
      }

      small {
        display: block;
        color: var(--tt-text-3);
      }
    }

    &__link {
      display: inline-block;
      margin-top: var(--tt-space-3);
      color: var(--tt-accent);
    }
  }
</style>
