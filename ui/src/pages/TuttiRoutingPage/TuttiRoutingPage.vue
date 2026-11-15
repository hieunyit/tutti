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
  /**
   * Routing rules: which storage serves a user's or bucket's requests. This
   * used to be the "Routing" tab of the Replication page; the list is the
   * unchanged RoutingTab (WP4c polishes it). The title comes from the route
   * name so the sidebar, breadcrumb, tab title and H1 always agree.
   */
  import { useI18n } from 'vue-i18n';
  import { NButton } from 'naive-ui';
  import RoutingTab from './RoutingTab.vue';
  import i18nReplicationHub from '@/pages/TuttiReplicationPage/i18nReplicationHub';
  import { RouteName } from '@/utils/types/router';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import { PageShell } from '@/components/kit';

  const { t } = useI18n({ messages: i18nReplicationHub });
  const { t: tRoute } = useI18n({ useScope: 'global' });
</script>

<template>
  <PageShell
    :title="tRoute(RouteName.TUTTI_ROUTING_RULES)"
    state="ready"
  >
    <template #actions>
      <WriteGate>
        <RouterLink :to="{ name: RouteName.TUTTI_ADD_ROUTING_POLICY }">
          <NButton
            type="primary"
            tag="span"
          >
            {{ t('addRoutingPolicy') }}
          </NButton>
        </RouterLink>
      </WriteGate>
    </template>

    <RoutingTab />
  </PageShell>
</template>
