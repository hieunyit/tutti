<!--
  - Copyright © 2026 Clyso GmbH
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
  import { onUnmounted, onBeforeMount } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { CButton, CIcon } from '@clyso/clyso-ui-kit';
  import PageHeader from '@/components/layout/PageHeader.vue';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import ReplicationsTile from '@/components/chorus/replications/ReplicationsTile/ReplicationsTile.vue';
  import i18nReplications from '@/components/chorus/replications/i18nReplications';
  import { useTuttiReplicationsStore } from '@/stores/tuttiReplicationsStore';
  import { IconName } from '@/utils/types/icon';
  import { RouteName } from '@/utils/types/router';

  const { t } = useI18n({ messages: i18nReplications });
  const { initReplicationsPage, $reset } = useTuttiReplicationsStore();

  onBeforeMount(() => {
    initReplicationsPage();
  });

  onUnmounted(() => {
    $reset();
  });
</script>

<template>
  <div class="replication-page">
    <PageHeader
      :title="t('replicationsTitle')"
      :description="t('pageDescription')"
    >
      <template #actions>
        <WriteGate>
          <RouterLink :to="{ name: RouteName.CHORUS_ADD_REPLICATION }">
            <CButton
              type="primary"
              tag="div"
            >
              <template #icon>
                <CIcon
                  :is-inline="true"
                  :name="IconName.BASE_ADD"
                />
              </template>

              {{ t('addReplicationAction') }}
            </CButton>
          </RouterLink>
        </WriteGate>
      </template>
    </PageHeader>

    <ReplicationsTile />
  </div>
</template>

<style lang="scss" scoped>
  .replication-page {
    height: 100%;
  }
</style>
