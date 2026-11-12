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
  import { CButton } from '@clyso/clyso-ui-kit';
  import { NButton, NCheckbox, NFormItem, NModal, NSelect } from 'naive-ui';
  import { storeToRefs } from 'pinia';
  import { computed, onBeforeMount, onBeforeUnmount, ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { useRoute, useRouter } from 'vue-router';
  import i18nProtection from '@/components/tutti/protection/i18nProtection';
  import ProtectionGroupDialog from '@/components/tutti/protection/ProtectionGroupDialog.vue';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import PageHeader from '@/components/layout/PageHeader.vue';
  import BackLink from '@/components/tutti/common/BackLink.vue';
  import ErrorState from '@/components/tutti/common/ErrorState.vue';
  import Callout from '@/components/ui/Callout.vue';
  import SectionCard from '@/components/ui/SectionCard.vue';
  import StatusTag from '@/components/ui/StatusTag.vue';
  import { EMPTY_VALUE, type StatusVariant } from '@/components/ui/status';
  import { useTuttiProtectionStore } from '@/stores/tuttiProtectionStore';
  import { formatDateTime } from '@/utils/helpers/DateTimeHelper';
  import {
    ProtectionGroupHelper,
    type MemberHealth,
  } from '@/utils/helpers/ProtectionGroupHelper';
  import { RouteName } from '@/utils/types/router';

  interface WriterRow {
    token: string;
    worker: boolean;
    owner: string;
    generation: number;
    instance: string;
    admittedAt: Date | null;
  }

  const props = defineProps<{
    name?: string;
  }>();

  const { t } = useI18n({ messages: i18nProtection });
  const route = useRoute();
  const router = useRouter();
  const store = useTuttiProtectionStore();
  const {
    detail,
    detailLoading,
    detailMissing,
    detailError,
    writers,
    writersError,
  } = storeToRefs(store);

  const groupId = computed(() => props.name || String(route.params.name ?? ''));
  const group = computed(() => detail.value?.group ?? null);

  function startDetailPolling() {
    if (!groupId.value) return;

    store.startPolling(() =>
      store.loadDetail(groupId.value, store.detail !== null),
    );
  }

  onBeforeMount(() => {
    store.resetDetail();
    startDetailPolling();
  });

  watch(groupId, () => {
    store.resetDetail();
    startDetailPolling();
  });

  onBeforeUnmount(() => {
    store.stopPolling();
    store.resetDetail();
  });

  const displayState = computed(() =>
    group.value ? ProtectionGroupHelper.displayState(group.value) : 'unknown',
  );

  const scopeLine = computed(() => {
    if (!group.value) return '';

    const kind = ProtectionGroupHelper.scopeKind(group.value);

    if (kind === 'storage') {
      return `${t('scopeStorageTitle')} · ${t('scopeStorage')}`;
    }

    const buckets = group.value.buckets ?? [];
    const scope = buckets.length ? buckets.join(', ') : t('scopeAllBuckets');

    return t('scopeLine', {
      user: group.value.user || EMPTY_VALUE,
      scope,
    });
  });

  const memberRows = computed(() =>
    group.value
      ? ProtectionGroupHelper.memberRows(group.value, detail.value ?? undefined)
      : [],
  );

  const switchSteps = computed(() =>
    group.value ? ProtectionGroupHelper.switchSteps(group.value) : [],
  );

  const isSwitchActive = computed(() =>
    switchSteps.value.some((step) => step.status !== 'pending'),
  );

  const historyRows = computed(() =>
    group.value ? ProtectionGroupHelper.historyNewestFirst(group.value) : [],
  );

  const writerRows = computed<WriterRow[]>(() => {
    const rows: WriterRow[] = [];

    for (const [token, record] of Object.entries(writers.value.writers ?? {})) {
      rows.push({
        token,
        worker: false,
        ...ProtectionGroupHelper.parseWriter(record),
      });
    }

    for (const [token, record] of Object.entries(writers.value.workers ?? {})) {
      rows.push({
        token,
        worker: true,
        ...ProtectionGroupHelper.parseWriter(record),
      });
    }

    return rows;
  });

  function healthVariant(health: MemberHealth): StatusVariant {
    switch (health) {
      case 'healthy':
        return 'success';

      case 'syncing':
        return 'info';

      case 'lagging':
        return 'warning';

      case 'unavailable':
        return 'danger';
    }
  }

  const afterSettingText = computed(() => {
    const seconds = ProtectionGroupHelper.afterSeconds(group.value?.after);

    return t('settingSeconds', { value: seconds ?? 30 });
  });

  // ---- dialogs ----
  const showEdit = ref(false);

  // Switchover dialog
  const showSwitch = ref(false);
  const switchTarget = ref<string | null>(null);
  const isSwitching = ref(false);
  const switchError = ref<string | null>(null);

  const switchCandidates = computed(() =>
    group.value ? ProtectionGroupHelper.switchTargets(group.value) : [],
  );
  const switchTargetOptions = computed(() =>
    switchCandidates.value.map((storage) => ({
      label: storage,
      value: storage,
    })),
  );
  const selectedTargetLag = computed<number>(() => {
    if (!switchTarget.value) return 0;

    const row = memberRows.value.find((r) => r.storage === switchTarget.value);

    return row?.lag ?? 0;
  });

  function openSwitchDialog() {
    switchTarget.value = switchCandidates.value[0] ?? null;
    switchError.value = null;
    isSwitching.value = false;
    showSwitch.value = true;
  }

  async function confirmSwitchover() {
    if (!group.value || !switchTarget.value) return;

    isSwitching.value = true;
    switchError.value = null;

    const err = await store.switchover(group.value.id, switchTarget.value);

    isSwitching.value = false;

    if (err === null) {
      showSwitch.value = false;

      return;
    }

    switchError.value = t('switchFailed', { message: err });
  }

  // Abort switchover dialog
  const showAbort = ref(false);
  const isAborting = ref(false);
  const abortError = ref<string | null>(null);

  function openAbortDialog() {
    abortError.value = null;
    isAborting.value = false;
    showAbort.value = true;
  }

  async function confirmAbort() {
    if (!group.value) return;

    isAborting.value = true;
    abortError.value = null;

    const err = await store.abortSwitch(group.value.id);

    isAborting.value = false;

    if (err === null) {
      showAbort.value = false;

      return;
    }

    abortError.value = t('abortFailed', { message: err });
  }

  // Detach group dialog
  const showDelete = ref(false);
  const isDeleting = ref(false);
  const deleteError = ref<string | null>(null);

  function openDeleteDialog() {
    deleteError.value = null;
    isDeleting.value = false;
    showDelete.value = true;
  }

  async function confirmDelete() {
    if (!group.value) return;

    isDeleting.value = true;
    deleteError.value = null;

    const err = await store.deleteGroup(group.value.id);

    isDeleting.value = false;

    if (err === null) {
      showDelete.value = false;
      await router.push({ name: RouteName.TUTTI_PROTECTION_GROUPS });

      return;
    }

    deleteError.value = t('deleteFailed', { message: err });
  }

  // Clear writer dialog
  const clearingWriter = ref<WriterRow | null>(null);
  const clearAttested = ref(false);
  const isClearing = ref(false);
  const clearError = ref<string | null>(null);

  function openClearWriter(row: WriterRow) {
    clearingWriter.value = row;
    clearAttested.value = false;
    clearError.value = null;
    isClearing.value = false;
  }

  async function confirmClearWriter() {
    if (!group.value || !clearingWriter.value || !clearAttested.value) return;

    isClearing.value = true;
    clearError.value = null;

    const err = await store.clearWriter(
      group.value.id,
      clearingWriter.value.token,
      clearingWriter.value.worker,
    );

    isClearing.value = false;

    if (err === null) {
      clearingWriter.value = null;

      return;
    }

    clearError.value = t('clearFailed', { message: err });
  }
</script>

<template>
  <div class="tutti-protection-group-detail-page">
    <BackLink
      :to="{ name: RouteName.TUTTI_PROTECTION_GROUPS }"
      :label="t('back')"
    />

    <ErrorState
      v-if="detailMissing"
      :title="t('notFound')"
      :text="t('notFoundText')"
      :action-label="t('back')"
      @retry="router.push({ name: RouteName.TUTTI_PROTECTION_GROUPS })"
    />

    <ErrorState
      v-else-if="detailError && !group"
      :title="t('detailError')"
      :text="t('errorText')"
      :action-label="t('retry')"
      @retry="store.loadDetail(groupId)"
    />

    <template v-else-if="group">
      <PageHeader
        :title="group.id"
        :description="scopeLine"
      >
        <template #actions>
          <CButton
            :loading="detailLoading"
            @click="store.loadDetail(groupId)"
          >
            {{ t('refresh') }}
          </CButton>
          <WriteGate>
            <div class="tutti-protection-group-detail-page__header-actions">
              <CButton
                secondary
                :disabled="!ProtectionGroupHelper.canEdit(group)"
                @click="showEdit = true"
              >
                {{ t('edit') }}
              </CButton>
              <CButton
                type="primary"
                :disabled="!ProtectionGroupHelper.canSwitch(group)"
                @click="openSwitchDialog"
              >
                {{ group.mode === 'auto' ? t('failoverNow') : t('switchover') }}
              </CButton>
              <CButton
                v-if="ProtectionGroupHelper.canAbort(group)"
                type="warning"
                secondary
                @click="openAbortDialog"
              >
                {{ t('cancelSwitch') }}
              </CButton>
              <CButton
                v-if="ProtectionGroupHelper.canDelete(group)"
                type="error"
                ghost
                @click="openDeleteDialog"
              >
                {{ t('deleteGroup') }}
              </CButton>
            </div>
          </WriteGate>
        </template>

        <template #extra>
          <div class="tutti-protection-group-detail-page__badges">
            <StatusTag
              :variant="ProtectionGroupHelper.stateVariant(displayState)"
              :label="t(`state.${displayState}`)"
            />
            <StatusTag
              variant="neutral"
              no-dot
              :label="t('generation', { value: group.generation ?? 0 })"
            />
            <StatusTag
              variant="neutral"
              no-dot
              :label="group.mode === 'auto' ? t('modeAuto') : t('modeManual')"
            />
          </div>
        </template>
      </PageHeader>

      <div class="tutti-protection-group-detail-page__content">
        <Callout
          v-if="group.frozen"
          tone="warning"
          :title="t('frozenBannerTitle')"
        >
          {{ t('frozenBanner') }}
        </Callout>

        <Callout
          v-if="(group.quarantined ?? []).length > 0"
          tone="warning"
        >
          {{
            t('quarantineBanner', {
              members: (group.quarantined ?? []).join(', '),
            })
          }}
        </Callout>

        <Callout
          v-if="group.lastError"
          tone="danger"
          :title="t('lastErrorTitle')"
        >
          <p class="tutti-protection-group-detail-page__error-msg">
            {{ group.lastError }}
          </p>
          <small class="tutti-protection-group-detail-page__error-hint">
            {{ t('lastErrorHint') }}
          </small>
        </Callout>

        <SectionCard
          v-if="isSwitchActive"
          :title="
            group.target
              ? t('progressTitle', { target: group.target })
              : t('progressGeneric')
          "
        >
          <ol class="tutti-protection-group-detail-page__steps">
            <li
              v-for="(step, idx) in switchSteps"
              :key="step.id"
              class="tutti-protection-group-detail-page__step"
              :class="`tutti-protection-group-detail-page__step--${step.status}`"
            >
              <span class="tutti-protection-group-detail-page__step-index">
                {{ step.status === 'done' ? '✓' : idx + 1 }}
              </span>
              <span class="tutti-protection-group-detail-page__step-label">
                {{ t(`step.${step.id}`) }}
              </span>
            </li>
          </ol>
        </SectionCard>

        <SectionCard
          :title="t('membersTitle')"
          flush
        >
          <div class="tt-table-wrap">
            <table class="tt-table">
              <thead>
                <tr>
                  <th scope="col">{{ t('colStorage') }}</th>
                  <th scope="col">{{ t('colRole') }}</th>
                  <th scope="col">{{ t('colHealth') }}</th>
                  <th
                    scope="col"
                    class="tt-num"
                  >
                    {{ t('colMemberLag') }}
                  </th>
                  <th scope="col">{{ t('colInitialSync') }}</th>
                  <th scope="col">{{ t('colExcluded') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="row in memberRows"
                  :key="row.storage"
                >
                  <td>
                    <strong>{{ row.storage }}</strong>
                  </td>
                  <td>
                    <StatusTag
                      v-if="row.isPrimary"
                      variant="info"
                      :label="t('rolePrimary')"
                    />
                    <StatusTag
                      v-else-if="row.isTarget"
                      variant="warning"
                      :label="t('roleTarget')"
                    />
                    <span v-else>{{ t('roleReplica') }}</span>
                  </td>
                  <td>
                    <StatusTag
                      :variant="healthVariant(row.health)"
                      :label="t(`health.${row.health}`)"
                    />
                  </td>
                  <td class="tt-num">
                    {{ ProtectionGroupHelper.lagText(row.lag) }}
                  </td>
                  <td>
                    {{
                      row.initialized === null
                        ? EMPTY_VALUE
                        : row.initialized
                          ? t('syncDone')
                          : t('syncRunning')
                    }}
                  </td>
                  <td>
                    <StatusTag
                      v-if="row.excluded"
                      variant="danger"
                      :label="t('excludedYes')"
                      :title="t('excludedHelp')"
                    />
                    <template v-else>{{ EMPTY_VALUE }}</template>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </SectionCard>

        <SectionCard :title="t('settingsTitle')">
          <dl class="tutti-protection-group-detail-page__facts">
            <div>
              <dt>{{ t('settingCopies') }}</dt>
              <dd>
                {{
                  t('copiesValue', {
                    healthy: detail?.healthyCopies ?? EMPTY_VALUE,
                    wanted: ProtectionGroupHelper.wantedCopies(group),
                  })
                }}
              </dd>
            </div>
            <div>
              <dt>{{ t('settingMode') }}</dt>
              <dd>
                {{ group.mode === 'auto' ? t('modeAuto') : t('modeManual') }}
              </dd>
            </div>
            <div>
              <dt>{{ t('settingAfter') }}</dt>
              <dd>{{ afterSettingText }}</dd>
            </div>
            <div>
              <dt>{{ t('settingLag') }}</dt>
              <dd>{{ group.maxEventLag ?? 0 }}</dd>
            </div>
            <div>
              <dt>{{ t('settingSwitches') }}</dt>
              <dd>{{ group.maxSwitchesPerHour ?? 3 }}</dd>
            </div>
            <div>
              <dt>{{ t('settingElectorate') }}</dt>
              <dd>
                {{
                  (group.electorate ?? []).length
                    ? (group.electorate ?? []).join(', ')
                    : EMPTY_VALUE
                }}
              </dd>
            </div>
          </dl>
        </SectionCard>

        <SectionCard
          :title="t('writersTitle')"
          :meta="t('writersCount', { count: writerRows.length })"
          flush
        >
          <div class="tutti-protection-group-detail-page__section-intro">
            {{ t('writersHelp') }}
          </div>

          <div
            v-if="writersError"
            class="tutti-protection-group-detail-page__section-pad"
          >
            <Callout tone="danger">{{ t('writersError') }}</Callout>
          </div>

          <p
            v-else-if="writerRows.length === 0"
            class="tutti-protection-group-detail-page__empty"
          >
            {{ t('writersEmpty') }}
          </p>

          <div
            v-else
            class="tt-table-wrap"
          >
            <table class="tt-table">
              <thead>
                <tr>
                  <th scope="col">{{ t('colWriterOwner') }}</th>
                  <th scope="col">{{ t('colWriterKind') }}</th>
                  <th scope="col">{{ t('colWriterInstance') }}</th>
                  <th scope="col">{{ t('colWriterSince') }}</th>
                  <th scope="col">
                    <span class="tt-sr-only">{{ t('clearWriter') }}</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="row in writerRows"
                  :key="`${row.worker ? 'w' : 'p'}-${row.token}`"
                >
                  <td>
                    <strong>{{ row.owner }}</strong>
                  </td>
                  <td>
                    {{ row.worker ? t('kindWorker') : t('kindProxy') }}
                  </td>
                  <td class="tt-mono">{{ row.instance || EMPTY_VALUE }}</td>
                  <td>{{ formatDateTime(row.admittedAt) }}</td>
                  <td class="tutti-protection-group-detail-page__row-actions">
                    <WriteGate>
                      <CButton
                        size="small"
                        type="error"
                        secondary
                        :disabled="!group.frozen"
                        :title="
                          !group.frozen ? t('clearOnlyFrozen') : undefined
                        "
                        @click="openClearWriter(row)"
                      >
                        {{ t('clearWriter') }}
                      </CButton>
                    </WriteGate>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </SectionCard>

        <SectionCard
          :title="t('historyTitle')"
          flush
        >
          <p
            v-if="historyRows.length === 0"
            class="tutti-protection-group-detail-page__empty"
          >
            {{ t('historyEmpty') }}
          </p>

          <div
            v-else
            class="tt-table-wrap"
          >
            <table class="tt-table">
              <thead>
                <tr>
                  <th scope="col">{{ t('colChange') }}</th>
                  <th scope="col">{{ t('switchover') }}</th>
                  <th scope="col">{{ t('colMode') }}</th>
                  <th scope="col">{{ t('colMemberLag') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="(event, idx) in historyRows"
                  :key="`${event.at ?? ''}-${idx}`"
                >
                  <td>{{ formatDateTime(event.at) }}</td>
                  <td>
                    <strong>{{
                      t('historyLine', {
                        from: event.from || EMPTY_VALUE,
                        to: event.to || EMPTY_VALUE,
                      })
                    }}</strong>
                  </td>
                  <td>
                    <StatusTag
                      :variant="event.automatic ? 'warning' : 'info'"
                      :label="
                        event.automatic
                          ? t('historyAutomatic')
                          : t('historyManual')
                      "
                    />
                  </td>
                  <td>
                    {{
                      t('historyMeta', {
                        generation: event.generation ?? 0,
                        lag: event.eventLag ?? 0,
                      })
                    }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </SectionCard>
      </div>

      <ProtectionGroupDialog
        v-model:show="showEdit"
        :group="group"
      />

      <!-- Switchover Modal -->
      <NModal
        :show="showSwitch"
        preset="card"
        :title="t('switchTitle', { id: group.id })"
        :closable="!isSwitching"
        :mask-closable="!isSwitching"
        :style="{ width: 'min(520px, calc(100vw - 32px))' }"
        @update:show="showSwitch = $event"
      >
        <div class="tutti-protection-group-detail-page__modal-body">
          <Callout
            v-if="switchCandidates.length === 0"
            tone="warning"
          >
            {{ t('switchNoTargets') }}
          </Callout>

          <template v-else>
            <NFormItem :label="t('switchPickTarget')">
              <NSelect
                v-model:value="switchTarget"
                :options="switchTargetOptions"
                :placeholder="t('switchPickPlaceholder')"
                :disabled="isSwitching"
              />
            </NFormItem>

            <p
              v-if="switchTarget"
              class="tutti-protection-group-detail-page__modal-text"
            >
              {{
                t('switchConsequence', {
                  target: switchTarget,
                  primary: group.primary || EMPTY_VALUE,
                })
              }}
            </p>

            <Callout
              v-if="switchTarget && selectedTargetLag > 0"
              tone="warning"
            >
              {{
                t('switchConsequenceLag', {
                  target: switchTarget,
                  lag: selectedTargetLag,
                })
              }}
            </Callout>
          </template>

          <Callout
            v-if="switchError"
            tone="danger"
          >
            {{ switchError }}
          </Callout>
        </div>

        <template #footer>
          <div class="tutti-protection-group-detail-page__modal-footer">
            <NButton
              :disabled="isSwitching"
              @click="showSwitch = false"
            >
              {{ t('cancel') }}
            </NButton>
            <NButton
              type="primary"
              :disabled="!switchTarget"
              :loading="isSwitching"
              @click="confirmSwitchover"
            >
              {{ t('switchConfirm', { target: switchTarget || '…' }) }}
            </NButton>
          </div>
        </template>
      </NModal>

      <!-- Abort Switchover Modal -->
      <NModal
        :show="showAbort"
        preset="card"
        :title="t('abortTitle')"
        :closable="!isAborting"
        :mask-closable="!isAborting"
        :style="{ width: 'min(480px, calc(100vw - 32px))' }"
        @update:show="showAbort = $event"
      >
        <div class="tutti-protection-group-detail-page__modal-body">
          <p class="tutti-protection-group-detail-page__modal-text">
            {{
              t('abortText', {
                target: group.target || EMPTY_VALUE,
                primary: group.primary || EMPTY_VALUE,
              })
            }}
          </p>
          <Callout
            v-if="abortError"
            tone="danger"
          >
            {{ abortError }}
          </Callout>
        </div>

        <template #footer>
          <div class="tutti-protection-group-detail-page__modal-footer">
            <NButton
              :disabled="isAborting"
              @click="showAbort = false"
            >
              {{ t('keepGoing') }}
            </NButton>
            <NButton
              type="warning"
              :loading="isAborting"
              @click="confirmAbort"
            >
              {{ t('abortConfirm') }}
            </NButton>
          </div>
        </template>
      </NModal>

      <!-- Detach Group Modal -->
      <NModal
        :show="showDelete"
        preset="card"
        :title="t('deleteTitle', { id: group.id })"
        :closable="!isDeleting"
        :mask-closable="!isDeleting"
        :style="{ width: 'min(480px, calc(100vw - 32px))' }"
        @update:show="showDelete = $event"
      >
        <div class="tutti-protection-group-detail-page__modal-body">
          <p class="tutti-protection-group-detail-page__modal-text">
            {{ t('deleteText') }}
          </p>
          <Callout
            v-if="deleteError"
            tone="danger"
          >
            {{ deleteError }}
          </Callout>
        </div>

        <template #footer>
          <div class="tutti-protection-group-detail-page__modal-footer">
            <NButton
              :disabled="isDeleting"
              @click="showDelete = false"
            >
              {{ t('cancel') }}
            </NButton>
            <NButton
              type="error"
              :loading="isDeleting"
              @click="confirmDelete"
            >
              {{ t('deleteConfirm') }}
            </NButton>
          </div>
        </template>
      </NModal>

      <!-- Clear Writer Modal -->
      <NModal
        :show="clearingWriter !== null"
        preset="card"
        :title="t('clearTitle', { owner: clearingWriter?.owner ?? '' })"
        :closable="!isClearing"
        :mask-closable="!isClearing"
        :style="{ width: 'min(520px, calc(100vw - 32px))' }"
        @update:show="!$event && (clearingWriter = null)"
      >
        <div class="tutti-protection-group-detail-page__modal-body">
          <p class="tutti-protection-group-detail-page__modal-text">
            {{ t('clearText') }}
          </p>
          <NCheckbox
            v-model:checked="clearAttested"
            :disabled="isClearing"
          >
            {{ t('clearAttest') }}
          </NCheckbox>
          <Callout
            v-if="clearError"
            tone="danger"
          >
            {{ clearError }}
          </Callout>
        </div>

        <template #footer>
          <div class="tutti-protection-group-detail-page__modal-footer">
            <NButton
              :disabled="isClearing"
              @click="clearingWriter = null"
            >
              {{ t('cancel') }}
            </NButton>
            <NButton
              type="error"
              :disabled="!clearAttested"
              :loading="isClearing"
              @click="confirmClearWriter"
            >
              {{ t('clearConfirm') }}
            </NButton>
          </div>
        </template>
      </NModal>
    </template>
  </div>
</template>

<style lang="scss" scoped>
  .tutti-protection-group-detail-page {
    display: flex;
    flex-direction: column;
    gap: 16px;
    min-width: 0;

    &__header-actions {
      display: flex;
      align-items: center;
      flex-wrap: wrap;
      gap: 8px;
    }

    &__badges {
      display: flex;
      align-items: center;
      flex-wrap: wrap;
      gap: 8px;
    }

    &__content {
      display: flex;
      flex-direction: column;
      gap: 20px;
    }

    &__error-msg {
      margin: 4px 0;
    }

    &__error-hint {
      color: var(--tt-text-3);
      font-size: 12px;
    }

    &__steps {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
      gap: 12px;
      margin: 0;
      padding: 0;
      list-style: none;
    }

    &__step {
      display: flex;
      align-items: center;
      gap: 10px;
      padding: 10px 12px;
      border: 1px solid var(--tt-border);
      border-radius: var(--tt-radius-control);
      background-color: var(--tt-subtle);
      color: var(--tt-text-3);
      font-size: 13px;

      &--done {
        border-color: var(--tt-success-border);
        background-color: var(--tt-success-bg);
        color: var(--tt-success);
      }

      &--active {
        border-color: var(--tt-info-border);
        background-color: var(--tt-info-bg);
        color: var(--tt-info);
        font-weight: 600;
      }
    }

    &__step-index {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: 22px;
      height: 22px;
      border-radius: 50%;
      border: 1px solid currentcolor;
      font-size: 12px;
      font-weight: 600;
      flex: none;
    }

    &__facts {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
      gap: 16px 24px;
      margin: 0;

      dt {
        color: var(--tt-text-3);
        font-size: 12px;
      }

      dd {
        margin: 4px 0 0;
        color: var(--tt-text);
        font-size: 14px;
        font-weight: 500;
        overflow-wrap: anywhere;
      }
    }

    &__section-intro {
      padding: 12px 16px;
      font-size: 13px;
      color: var(--tt-text-2);
      border-bottom: 1px solid var(--tt-border-subtle);
    }

    &__section-pad {
      padding: 16px;
    }

    &__empty {
      margin: 0;
      padding: 16px;
      font-size: 13px;
      color: var(--tt-text-3);
    }

    &__row-actions {
      text-align: right;
    }

    &__modal-body {
      display: flex;
      flex-direction: column;
      gap: 12px;
    }

    &__modal-text {
      margin: 0;
      font-size: 14px;
      line-height: 20px;
      color: var(--tt-text-2);
    }

    &__modal-footer {
      display: flex;
      justify-content: flex-end;
      gap: 8px;
    }
  }
</style>
