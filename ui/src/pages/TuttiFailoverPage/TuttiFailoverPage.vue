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
  import { CButton, CIcon } from '@clyso/clyso-ui-kit';
  import { storeToRefs } from 'pinia';
  import { computed, onBeforeMount, onBeforeUnmount, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nFailover from '@/components/tutti/failover/i18nFailover';
  import FailoverPoliciesTile from '@/components/tutti/failover/FailoverPoliciesTile.vue';
  import FailoverEventsTile from '@/components/tutti/failover/FailoverEventsTile.vue';
  import PageHeader from '@/components/layout/PageHeader.vue';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import Callout from '@/components/ui/Callout.vue';
  import KpiCard from '@/components/ui/KpiCard.vue';
  import { EMPTY_VALUE } from '@/components/ui/status';
  import ErrorState from '@/components/tutti/common/ErrorState.vue';
  import { useTuttiFailoverStore } from '@/stores/tuttiFailoverStore';
  import { IconName } from '@/utils/types/icon';
  import { RouteName } from '@/utils/types/router';

  const { t } = useI18n({ messages: i18nFailover });
  const store = useTuttiFailoverStore();
  const { hasError, isLoading, readiness, policies, events } =
    storeToRefs(store);
  const policiesTile = ref<InstanceType<typeof FailoverPoliciesTile> | null>(
    null,
  );

  const replicationLabel = computed(
    () =>
      ({
        paused: t('kpiReplicationPaused'),
        lagging: t('kpiReplicationLagging'),
        inSync: t('kpiReplicationInSync'),
        none: EMPTY_VALUE,
      })[readiness.value.replication],
  );
  const policySummary = computed(() => {
    const enabled = policies.value.filter((policy) => policy.enabled !== false);

    return enabled.length
      ? enabled
          .map(
            (policy) =>
              `${policy.source} → ${(policy.targets ?? [])[0] ?? EMPTY_VALUE}`,
          )
          .join(', ')
      : t('kpiPoliciesNone');
  });

  onBeforeMount(() => store.init());
  onBeforeUnmount(() => store.$reset());
</script>

<template>
  <div class="tutti-failover-page">
    <PageHeader
      :title="t('title')"
      :description="t('pageDescription')"
    >
      <template #actions>
        <CButton
          :loading="isLoading"
          @click="store.init()"
        >
          {{ t('refresh') }}
        </CButton>
        <WriteGate v-if="!hasError">
          <CButton
            type="primary"
            @click="policiesTile?.openAdd()"
          >
            <template #icon>
              <CIcon
                :is-inline="true"
                :name="IconName.BASE_ADD"
              />
            </template>

            {{ t('addPolicy') }}
          </CButton>
        </WriteGate>
      </template>
    </PageHeader>

    <ErrorState
      v-if="hasError"
      :title="t('errorTitle')"
      :text="t('errorText')"
      :action-label="t('retry')"
      @retry="store.init()"
    />
    <div
      v-else
      class="tutti-failover-page__content"
    >
      <Callout
        tone="info"
        :title="t('legacyBannerTitle')"
      >
        <p class="tutti-failover-page__banner-text">
          {{ t('legacyBannerText') }}
        </p>
        <RouterLink :to="{ name: RouteName.TUTTI_PROTECTION_GROUPS }">{{
          t('title')
        }}</RouterLink>
      </Callout>

      <Callout
        v-if="readiness.willNotTrigger"
        tone="danger"
        :title="
          t('bannerTitle', {
            off: readiness.off.length,
            total: readiness.total,
          })
        "
      >
        <p class="tutti-failover-page__banner-text">{{ t('bannerText') }}</p>
        <div class="tutti-failover-page__banner-meta">
          <RouterLink :to="{ name: RouteName.TUTTI_DOCS }">{{
            t('bannerDocs')
          }}</RouterLink>
          <span
            v-for="proxy in readiness.off"
            :key="proxy.appId"
            >{{
              t('bannerProxy', {
                id: proxy.appId,
                interval: proxy.checkInterval ?? EMPTY_VALUE,
              })
            }}</span
          >
        </div>
      </Callout>

      <section
        class="tutti-failover-page__kpis"
        :aria-label="t('readiness')"
      >
        <KpiCard
          :label="t('kpiProxies')"
          :value="`${readiness.on} / ${readiness.total}`"
          :sub="t('kpiProxiesSub')"
          :tone="
            readiness.total > 0 && readiness.on < readiness.total
              ? 'danger'
              : 'default'
          "
        />
        <KpiCard
          :label="t('kpiPolicies')"
          :value="readiness.enabledPolicies"
          :sub="policySummary"
        />
        <KpiCard
          :label="t('kpiReplication')"
          :value="replicationLabel"
          :sub="t('kpiReplicationSub')"
          :tone="
            readiness.replication === 'paused' ||
            readiness.replication === 'lagging'
              ? 'warning'
              : 'default'
          "
        />
        <KpiCard
          :label="t('kpiEvents')"
          :value="events.length"
          :sub="t('kpiEventsSub')"
        />
      </section>

      <FailoverPoliciesTile ref="policiesTile" />
      <FailoverEventsTile />
    </div>
  </div>
</template>

<style lang="scss" scoped>
  .tutti-failover-page {
    &__content {
      display: flex;
      flex-direction: column;
      gap: 20px;
    }

    &__kpis {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
      gap: 16px;
    }

    &__banner-text {
      margin: 4px 0 6px;
    }

    &__banner-meta {
      display: flex;
      flex-wrap: wrap;
      gap: 4px 12px;
    }
  }
</style>
