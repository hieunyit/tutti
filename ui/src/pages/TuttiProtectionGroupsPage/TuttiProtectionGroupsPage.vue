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
  import { onBeforeMount, onBeforeUnmount, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { useRouter } from 'vue-router';
  import i18nProtection from '@/components/tutti/protection/i18nProtection';
  import ProtectionGroupDialog from '@/components/tutti/protection/ProtectionGroupDialog.vue';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import PageHeader from '@/components/layout/PageHeader.vue';
  import ConfirmDialog from '@/components/tutti/common/ConfirmDialog.vue';
  import ErrorState from '@/components/tutti/common/ErrorState.vue';
  import Callout from '@/components/ui/Callout.vue';
  import EmptyState from '@/components/ui/EmptyState.vue';
  import KpiCard from '@/components/ui/KpiCard.vue';
  import SectionCard from '@/components/ui/SectionCard.vue';
  import StatusTag from '@/components/ui/StatusTag.vue';
  import { EMPTY_VALUE } from '@/components/ui/status';
  import { useTuttiProtectionStore } from '@/stores/tuttiProtectionStore';
  import { formatDateTime } from '@/utils/helpers/DateTimeHelper';
  import { ProtectionGroupHelper } from '@/utils/helpers/ProtectionGroupHelper';
  import { IconName } from '@/utils/types/icon';
  import type { ProtectionGroup } from '@/utils/types/protection';
  import { RouteName } from '@/utils/types/router';

  const { t } = useI18n({ messages: i18nProtection });
  const router = useRouter();
  const store = useTuttiProtectionStore();
  const {
    isLoading,
    hasError,
    groups,
    statuses,
    automaticFrozen,
    summary,
  } = storeToRefs(store);

  const showCreate = ref(false);
  const showFreezeConfirm = ref(false);
  const isFreezing = ref(false);
  const freezeError = ref<string | null>(null);

  onBeforeMount(() => store.startPolling());
  onBeforeUnmount(() => store.stopPolling());

  function scopeText(group: ProtectionGroup): string {
    const kind = ProtectionGroupHelper.scopeKind(group);

    if (kind === 'user') return t('scopeUser');

    const count = (group.buckets ?? []).length;

    return count === 1 ? t('scopeBucketsOne') : t('scopeBuckets', { count });
  }

  function copiesText(group: ProtectionGroup): string {
    const status = statuses.value[group.id];
    const healthy = status?.healthyCopies ?? EMPTY_VALUE;
    const wanted = ProtectionGroupHelper.wantedCopies(group);

    return t('copiesValue', { healthy, wanted });
  }

  function maxLagText(group: ProtectionGroup): string {
    const lag = ProtectionGroupHelper.maxLag(statuses.value[group.id]);

    return ProtectionGroupHelper.lagText(lag);
  }

  function lastChangeText(group: ProtectionGroup): string {
    const latest = ProtectionGroupHelper.historyNewestFirst(group)[0];

    return formatDateTime(latest?.at);
  }

  function openGroup(group: ProtectionGroup) {
    router.push({
      name: RouteName.TUTTI_PROTECTION_GROUP_DETAIL,
      params: { name: group.id },
    });
  }

  async function confirmToggleFreeze() {
    const targetFrozen = !automaticFrozen.value;

    isFreezing.value = true;
    freezeError.value = null;

    const err = await store.setAutomaticFreeze(targetFrozen);

    isFreezing.value = false;

    if (err === null) {
      showFreezeConfirm.value = false;

      return;
    }

    freezeError.value = t('freezeFailed');
    showFreezeConfirm.value = false;
  }
</script>

<template>
  <div class="tutti-protection-groups-page">
    <PageHeader
      :title="t('title')"
      :description="t('pageDescription')"
    >
      <template #actions>
        <CButton
          :loading="isLoading"
          @click="store.refresh()"
        >
          {{ t('refresh') }}
        </CButton>
        <WriteGate v-if="!hasError">
          <CButton
            type="primary"
            @click="showCreate = true"
          >
            <template #icon>
              <CIcon
                :is-inline="true"
                :name="IconName.BASE_ADD"
              />
            </template>

            {{ t('create') }}
          </CButton>
        </WriteGate>
      </template>
    </PageHeader>

    <ErrorState
      v-if="hasError"
      :title="t('errorTitle')"
      :text="t('errorText')"
      :action-label="t('retry')"
      @retry="store.refresh()"
    />

    <div
      v-else
      class="tutti-protection-groups-page__content"
    >
      <section
        class="tutti-protection-groups-page__kpis"
        :aria-label="t('title')"
      >
        <KpiCard
          :label="t('kpiGroups')"
          :value="summary.total"
          :sub="t('kpiGroupsSub')"
        />
        <KpiCard
          :label="t('kpiHealthy')"
          :value="summary.healthy"
          :sub="t('kpiHealthySub')"
          :tone="summary.healthy > 0 ? 'success' : 'default'"
        />
        <KpiCard
          :label="t('kpiDegraded')"
          :value="summary.degraded"
          :sub="t('kpiDegradedSub')"
          :tone="summary.degraded > 0 ? 'warning' : 'default'"
        />
        <KpiCard
          :label="t('kpiFrozen')"
          :value="summary.frozen"
          :sub="t('kpiFrozenSub')"
          :tone="summary.frozen > 0 ? 'warning' : 'default'"
        />
      </section>

      <Callout
        v-if="automaticFrozen !== null"
        :tone="automaticFrozen ? 'warning' : 'info'"
        :title="
          `${t('freezeTitle')} · ${
            automaticFrozen ? t('freezeOn') : t('freezeOff')
          }`
        "
      >
        <p class="tutti-protection-groups-page__freeze-help">
          {{ t('freezeHelp') }}
        </p>
        <template #actions>
          <WriteGate>
            <CButton
              size="small"
              :type="automaticFrozen ? 'primary' : 'warning'"
              secondary
              @click="showFreezeConfirm = true"
            >
              {{
                automaticFrozen
                  ? t('freezeDisableAction')
                  : t('freezeEnableAction')
              }}
            </CButton>
          </WriteGate>
        </template>
      </Callout>

      <Callout
        v-if="freezeError"
        tone="danger"
      >
        {{ freezeError }}
      </Callout>

      <SectionCard
        :title="t('tableTitle')"
        flush
      >
        <EmptyState
          v-if="groups.length === 0"
          :icon="IconName.BASE_SERVER"
          :title="t('emptyTitle')"
          :text="t('emptyText')"
        >
          <WriteGate>
            <CButton
              type="primary"
              @click="showCreate = true"
            >
              <template #icon>
                <CIcon
                  :is-inline="true"
                  :name="IconName.BASE_ADD"
                />
              </template>

              {{ t('create') }}
            </CButton>
          </WriteGate>
        </EmptyState>

        <div
          v-else
          class="tt-table-wrap"
        >
          <table class="tt-table">
            <thead>
              <tr>
                <th scope="col">{{ t('colName') }}</th>
                <th scope="col">{{ t('colScope') }}</th>
                <th scope="col">{{ t('colMembers') }}</th>
                <th
                  scope="col"
                  class="tt-num"
                >
                  {{ t('colCopies') }}
                </th>
                <th
                  scope="col"
                  class="tt-num"
                >
                  {{ t('colLag') }}
                </th>
                <th scope="col">{{ t('colState') }}</th>
                <th scope="col">{{ t('colMode') }}</th>
                <th scope="col">{{ t('colChange') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="group in groups"
                :key="group.id"
                class="tutti-protection-groups-page__row"
                @click="openGroup(group)"
              >
                <td>
                  <RouterLink
                    class="tutti-protection-groups-page__link"
                    :to="{
                      name: RouteName.TUTTI_PROTECTION_GROUP_DETAIL,
                      params: { name: group.id },
                    }"
                    @click.stop
                  >
                    <strong>{{ group.id }}</strong>
                  </RouterLink>
                </td>
                <td>
                  <div>{{ group.user || EMPTY_VALUE }}</div>
                  <small class="tutti-protection-groups-page__sub">{{
                    scopeText(group)
                  }}</small>
                </td>
                <td>
                  <span class="tutti-protection-groups-page__chips">
                    <span
                      v-for="member in group.members ?? []"
                      :key="member"
                      class="tutti-protection-groups-page__chip"
                      :class="{
                        'tutti-protection-groups-page__chip--primary':
                          member === group.primary,
                      }"
                    >
                      {{ member }}
                      <em
                        v-if="member === group.primary"
                        class="tutti-protection-groups-page__primary-mark"
                      >
                        ({{ t('primaryMark') }})
                      </em>
                    </span>
                    <template v-if="!(group.members ?? []).length">
                      {{ EMPTY_VALUE }}
                    </template>
                  </span>
                </td>
                <td class="tt-num">{{ copiesText(group) }}</td>
                <td class="tt-num">{{ maxLagText(group) }}</td>
                <td>
                  <StatusTag
                    :variant="
                      ProtectionGroupHelper.stateVariant(
                        ProtectionGroupHelper.displayState(group),
                      )
                    "
                    :label="
                      t(`state.${ProtectionGroupHelper.displayState(group)}`)
                    "
                  />
                </td>
                <td>
                  {{ group.mode === 'auto' ? t('modeAuto') : t('modeManual') }}
                </td>
                <td>{{ lastChangeText(group) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </SectionCard>
    </div>

    <ProtectionGroupDialog
      v-model:show="showCreate"
      :group="null"
    />

    <ConfirmDialog
      :show="showFreezeConfirm"
      :title="
        automaticFrozen ? t('freezeDisableTitle') : t('freezeEnableTitle')
      "
      :text="automaticFrozen ? t('freezeDisableText') : t('freezeEnableText')"
      :confirm-label="
        automaticFrozen ? t('freezeDisableAction') : t('freezeEnableAction')
      "
      :cancel-label="t('cancel')"
      :danger="!automaticFrozen"
      :loading="isFreezing"
      @update:show="showFreezeConfirm = $event"
      @confirm="confirmToggleFreeze"
    />
  </div>
</template>

<style lang="scss" scoped>
  .tutti-protection-groups-page {
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

    &__freeze-help {
      margin: 4px 0 0;
    }

    &__row {
      cursor: pointer;

      &:hover {
        background-color: var(--tt-hover);
      }
    }

    &__link {
      color: var(--tt-accent);
      text-decoration: none;

      &:hover {
        text-decoration: underline;
      }
    }

    &__sub {
      display: block;
      font-size: 12px;
      color: var(--tt-text-3);
    }

    &__chips {
      display: inline-flex;
      flex-wrap: wrap;
      gap: 4px;
    }

    &__chip {
      padding: 1px 8px;
      font-size: 12px;
      border: 1px solid var(--tt-border-strong);
      border-radius: var(--tt-radius-tag);

      &--primary {
        border-color: var(--tt-accent);
        background-color: var(--tt-accent-subtle);
      }
    }

    &__primary-mark {
      font-style: normal;
      font-weight: 600;
      color: var(--tt-accent);
    }
  }
</style>
